package node

// bid.go — Two-phase commit (2PC) bid proposal logic and Ricart-Agrawala
// critical-section integration.

import (
	"fmt"
	"log"
	"strings"
	"time"
)

// ProposeBid runs the full 2PC bid protocol as coordinator.
func (n *Node) ProposeBid(amount int, bidder string) (bool, string) {
	txnBid := BidArgs{Amount: amount, Bidder: bidder}
	if !n.canPrepareBid(txnBid) {
		return false, "Bid must be higher than current highest bid (or auction inactive)"
	}

	n.RA.RequestCS()
	defer n.RA.ReleaseCS()

	// Re-check after acquiring the critical section
	if !n.canPrepareBid(txnBid) {
		return false, "Bid became stale during coordination"
	}

	txnID := fmt.Sprintf("%s-%d", n.ID, n.Clock.Tick())
	quorum := (len(n.Peers)+1)/2 + 1
	votes := 1
	n.logTxnEvent(txnID, "TXN_BEGIN", fmt.Sprintf("bid=%d bidder=%s quorum=%d", amount, bidder, quorum))

	n.rememberPendingTxn(txnID, txnBid)

	type voteResult struct{ yes bool }
	voteCh := make(chan voteResult, len(n.Peers))

	// Phase 1: Prepare — ask all peers to vote
	for _, peer := range n.Peers {
		go func(p string) {
			var vote PrepareReply
			err := n.Client.Call(p, "NodeRPC.PrepareBid",
				PrepareArgs{TxnID: txnID, Bid: txnBid, Timestamp: n.Clock.Tick()}, &vote)
			if err != nil {
				voteCh <- voteResult{yes: false}
				return
			}
			voteCh <- voteResult{yes: vote.Vote}
		}(peer)
	}

	// Collect votes with a timeout
	pendingResponses := len(n.Peers)
	voteTimer := time.NewTimer(voteWaitTimeout)
	for pendingResponses > 0 {
		if votes >= quorum || votes+pendingResponses < quorum {
			break
		}
		select {
		case result := <-voteCh:
			pendingResponses--
			if result.yes {
				votes++
			}
		case <-voteTimer.C:
			pendingResponses = 0
		}
	}
	if !voteTimer.Stop() {
		select {
		case <-voteTimer.C:
		default:
		}
	}

	// Phase 2: Decide — apply locally and broadcast decision
	commit := votes >= quorum
	n.applyDecision(txnID, commit, txnBid)

	decision := DecisionArgs{TxnID: txnID, Commit: commit, Bid: txnBid, Leader: n.ID}
	if !commit {
		n.logTxnEvent(txnID, "TXN_ABORT", fmt.Sprintf("votes=%d quorum=%d", votes, quorum))
		for _, peer := range n.Peers {
			go func(p string) {
				var ack bool
				_ = n.Client.Call(p, "NodeRPC.DecideBid", decision, &ack)
			}(peer)
		}
		log.Printf("[%s] Txn %s aborted (votes=%d, quorum=%d)\n", n.ID, txnID, votes, quorum)
		return false, fmt.Sprintf("Bid aborted: quorum not reached (%d/%d)", votes, quorum)
	}

	ackCount, allAcked, missingPeers := n.broadcastDecisionAndCollectAcks(txnID, decision)

	go n.broadcastQueueState()
	// Anti-snipe: if a bid lands with less than 15s left, extend the deadline.
	n.maybeExtendDeadline()
	log.Printf("[%s] Txn %s committed bid=%d bidder=%s\n", n.ID, txnID, amount, bidder)

	if allAcked {
		n.logTxnEvent(txnID, "TXN_TERMINATED", fmt.Sprintf("all participants ACKed (%d/%d)", ackCount, len(n.Peers)))
		return true, "Bid committed by quorum and globally terminated"
	}

	n.logTxnEvent(txnID, "TXN_TERMINATION_PENDING", fmt.Sprintf("ACKs=%d/%d missing=%s", ackCount, len(n.Peers), strings.Join(missingPeers, ",")))
	go n.retryDecisionUntilAllAcked(txnID, decision, missingPeers)
	return true, fmt.Sprintf("Bid committed by quorum; waiting for participant ACKs (%d/%d)", ackCount, len(n.Peers))
}

// canPrepareBid checks whether a bid is valid against current queue state.
func (n *Node) canPrepareBid(bid BidArgs) bool {
	n.Queue.mu.Lock()
	defer n.Queue.mu.Unlock()
	return n.Queue.Active &&
		n.Queue.CurrentItem != nil &&
		bid.Amount > n.Queue.CurrentHighestBid &&
		time.Now().Unix() < n.Queue.DeadlineUnix
}

// rememberPendingTxn stores a prepared-but-not-yet-decided transaction.
func (n *Node) rememberPendingTxn(txnID string, bid BidArgs) {
	n.TxnMutex.Lock()
	n.PendingTxns[txnID] = PendingTxn{Bid: bid, PreparedAt: time.Now()}
	n.TxnMutex.Unlock()
	n.logTxnEvent(txnID, "TXN_PREPARED", fmt.Sprintf("bid=%d bidder=%s", bid.Amount, bid.Bidder))
}

// applyDecision commits or aborts a transaction and updates queue state.
func (n *Node) applyDecision(txnID string, commit bool, fallbackBid BidArgs) {
	n.TxnMutex.Lock()
	pending, ok := n.PendingTxns[txnID]
	bid := pending.Bid
	if !ok {
		bid = fallbackBid
	}
	delete(n.PendingTxns, txnID)
	n.TxnMutex.Unlock()

	if !commit {
		n.logTxnEvent(txnID, "TXN_ABORT_APPLIED", fmt.Sprintf("bid=%d bidder=%s", bid.Amount, bid.Bidder))
		return
	}

	n.Queue.mu.Lock()
	if n.Queue.Active && n.Queue.CurrentItem != nil && bid.Amount > n.Queue.CurrentHighestBid {
		n.Queue.CurrentHighestBid = bid.Amount
		n.Queue.CurrentWinner = bid.Bidder
	}
	n.Queue.mu.Unlock()
	n.logTxnEvent(txnID, "TXN_COMMIT_APPLIED", fmt.Sprintf("bid=%d bidder=%s", bid.Amount, bid.Bidder))
}

// abortStalePreparedTxns cleans up transactions that never received a decision (2PC timeout).
func (n *Node) abortStalePreparedTxns() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		n.TxnMutex.Lock()
		for txnID, pending := range n.PendingTxns {
			if now.Sub(pending.PreparedAt) > preparedTxnTTL {
				delete(n.PendingTxns, txnID)
				log.Printf("[%s] Auto-aborted stale txn %s\n", n.ID, txnID)
				n.logTxnEvent(txnID, "TXN_STALE_ABORT", "prepared txn timed out before decision")
			}
		}
		n.TxnMutex.Unlock()
	}
}

func (n *Node) broadcastDecisionAndCollectAcks(txnID string, decision DecisionArgs) (int, bool, []string) {
	if len(n.Peers) == 0 {
		return 0, true, nil
	}

	type ackResult struct {
		peer string
		ack  bool
	}
	ackCh := make(chan ackResult, len(n.Peers))
	missing := make(map[string]bool, len(n.Peers))
	for _, peer := range n.Peers {
		missing[peer] = true
		go func(p string) {
			var ack bool
			err := n.Client.Call(p, "NodeRPC.DecideBid", decision, &ack)
			ackCh <- ackResult{peer: p, ack: err == nil && ack}
		}(peer)
	}

	acks := 0
	pending := len(n.Peers)
	timer := time.NewTimer(decisionAckWaitTimeout)
	defer timer.Stop()

	for pending > 0 {
		select {
		case result := <-ackCh:
			pending--
			if result.ack {
				acks++
				delete(missing, result.peer)
				n.logTxnEvent(txnID, "TXN_DECIDE_ACK", fmt.Sprintf("peer=%s", result.peer))
			}
		case <-timer.C:
			pending = 0
		}
	}

	missingPeers := make([]string, 0, len(missing))
	for peer := range missing {
		missingPeers = append(missingPeers, peer)
	}
	return acks, len(missingPeers) == 0, missingPeers
}

func (n *Node) retryDecisionUntilAllAcked(txnID string, decision DecisionArgs, missing []string) {
	remaining := append([]string(nil), missing...)
	for attempt := 1; attempt <= decisionAckMaxRetries && len(remaining) > 0; attempt++ {
		time.Sleep(decisionAckRetryInterval)
		nextRemaining := make([]string, 0, len(remaining))
		for _, peer := range remaining {
			var ack bool
			err := n.Client.Call(peer, "NodeRPC.DecideBid", decision, &ack)
			if err == nil && ack {
				n.logTxnEvent(txnID, "TXN_DECIDE_ACK_RETRY", fmt.Sprintf("peer=%s attempt=%d", peer, attempt))
				continue
			}
			nextRemaining = append(nextRemaining, peer)
		}
		remaining = nextRemaining
		if len(remaining) > 0 {
			n.logTxnEvent(txnID, "TXN_TERMINATION_RETRY", fmt.Sprintf("attempt=%d remaining=%s", attempt, strings.Join(remaining, ",")))
		}
	}

	if len(remaining) == 0 {
		n.logTxnEvent(txnID, "TXN_TERMINATED", "all participants ACKed after retry")
		return
	}
	n.logTxnEvent(txnID, "TXN_TERMINATION_INCOMPLETE", fmt.Sprintf("unacked participants=%s", strings.Join(remaining, ",")))
}
