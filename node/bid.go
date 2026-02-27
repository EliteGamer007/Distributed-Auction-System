package node

// bid.go — Two-phase commit (2PC) bid proposal logic and Ricart-Agrawala
// critical-section integration.

import (
	"fmt"
	"log"
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
	for _, peer := range n.Peers {
		go func(p string) {
			var ack bool
			_ = n.Client.Call(p, "NodeRPC.DecideBid", decision, &ack)
		}(peer)
	}

	if commit {
		go n.broadcastQueueState()
		log.Printf("[%s] Txn %s committed bid=%d bidder=%s\n", n.ID, txnID, amount, bidder)
		return true, "Bid committed by quorum"
	}

	log.Printf("[%s] Txn %s aborted (votes=%d, quorum=%d)\n", n.ID, txnID, votes, quorum)
	return false, fmt.Sprintf("Bid aborted: quorum not reached (%d/%d)", votes, quorum)
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
		return
	}

	n.Queue.mu.Lock()
	if n.Queue.Active && n.Queue.CurrentItem != nil && bid.Amount > n.Queue.CurrentHighestBid {
		n.Queue.CurrentHighestBid = bid.Amount
		n.Queue.CurrentWinner = bid.Bidder
	}
	n.Queue.mu.Unlock()
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
			}
		}
		n.TxnMutex.Unlock()
	}
}
