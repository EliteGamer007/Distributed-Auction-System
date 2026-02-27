package node

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	voteWaitTimeout = 2500 * time.Millisecond
	preparedTxnTTL  = 8 * time.Second
)

type Node struct {
	ID            string
	Address       string
	Peers         []string
	State         *AuctionState
	Clock         *LamportClock
	RA            *RAManager
	Client        *RPCClient
	Rank          int
	Coordinator   string
	ElectionMutex sync.Mutex
	LeaderChan    chan bool // True for receiving heartbeat
	TxnMutex      sync.Mutex
	PendingTxns   map[string]PendingTxn
}

type PendingTxn struct {
	Bid        BidArgs
	PreparedAt time.Time
}

type NodeRPC struct {
	node *Node
}

func NewNode(id, address string, peers []string, rank int) *Node {
	clock := &LamportClock{}
	client := &RPCClient{}
	ra := NewRAManager(id, peers, clock, client)
	state := &AuctionState{Active: true}

	return &Node{
		ID:          id,
		Address:     address,
		Peers:       peers,
		State:       state,
		Clock:       clock,
		RA:          ra,
		Client:      client,
		Rank:        rank,
		LeaderChan:  make(chan bool),
		PendingTxns: map[string]PendingTxn{},
	}
}

func (n *Node) Start() {
	rpcServer := &NodeRPC{node: n}
	server := rpc.NewServer()
	_ = server.Register(rpcServer)

	listener, err := net.Listen("tcp", n.Address)
	if err != nil {
		log.Fatalf("Listen error: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle(rpc.DefaultRPCPath, server)

	mux.HandleFunc("/", n.handleUI)
	mux.HandleFunc("/bid", n.handleBidRequest)
	mux.HandleFunc("/state", n.handleStateRequest)

	go func() {
		if err := http.Serve(listener, mux); err != nil {
			log.Printf("HTTP server error on %s: %v", n.Address, err)
		}
	}()
	go n.abortStalePreparedTxns()
	go n.periodicStateSync()
	log.Printf("Node %s listening on %s (UI at http://%s)\n", n.ID, n.Address, n.Address)
}

func (n *Node) handleUI(w http.ResponseWriter, r *http.Request) {
	html := fmt.Sprintf(`
		<!DOCTYPE html>
		<html lang="en">
		<head>
			<meta charset="UTF-8">
			<meta name="viewport" content="width=device-width, initial-scale=1.0">
			<title>Auction Node %s</title>
			<link rel="preconnect" href="https://fonts.googleapis.com">
			<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
			<link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600&display=swap" rel="stylesheet">
			<style>
				:root {
					--bg-color: #ffffff;
					--text-color: #000000;
					--accent-color: #1d1d1f;
					--secondary-text: #86868b;
					--card-bg: #f5f5f7;
					--input-bg: rgba(0, 0, 0, 0.05);
					--success: #00c853;
					--error: #ff5252;
					--transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
				}

				@media (prefers-color-scheme: dark) {
					:root {
						--bg-color: #000000;
						--text-color: #f5f5f7;
						--accent-color: #ffffff;
						--secondary-text: #86868b;
						--card-bg: #1d1d1f;
						--input-bg: rgba(255, 255, 255, 0.1);
					}
				}

				* {
					margin: 0;
					padding: 0;
					box-sizing: border-box;
					font-family: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif;
					-webkit-font-smoothing: antialiased;
				}

				body {
					background-color: var(--bg-color);
					color: var(--text-color);
					display: flex;
					justify-content: center;
					align-items: center;
					min-height: 100vh;
					padding: 20px;
					transition: var(--transition);
				}

				.container {
					width: 100%%;
					max-width: 440px;
					animation: fadeIn 0.8s ease-out;
				}

				@keyframes fadeIn {
					from { opacity: 0; transform: translateY(20px); }
					to { opacity: 1; transform: translateY(0); }
				}

				header {
					text-align: center;
					margin-bottom: 40px;
				}

				h1 {
					font-size: 2.4rem;
					font-weight: 600;
					letter-spacing: -0.02em;
					margin-bottom: 8px;
				}

				.node-badge {
					display: inline-block;
					font-size: 0.9rem;
					font-weight: 500;
					color: var(--secondary-text);
					text-transform: uppercase;
					letter-spacing: 0.1em;
				}

				.card {
					background-color: var(--card-bg);
					border-radius: 28px;
					padding: 32px;
					box-shadow: 0 4px 24px rgba(0, 0, 0, 0.04);
					margin-bottom: 24px;
					transition: var(--transition);
				}

				.stat-group {
					margin-bottom: 24px;
				}

				.stat-label {
					font-size: 0.85rem;
					font-weight: 500;
					color: var(--secondary-text);
					margin-bottom: 4px;
				}

				.stat-value {
					font-size: 1.8rem;
					font-weight: 600;
					color: var(--text-color);
				}

				.status-dot {
					display: inline-block;
					width: 8px;
					height: 8px;
					border-radius: 50%%;
					margin-right: 8px;
					background-color: var(--success);
				}

				.status-text {
					font-size: 1rem;
					font-weight: 500;
				}

				form {
					display: flex;
					flex-direction: column;
					gap: 16px;
				}

				.input-group {
					position: relative;
				}

				input[type="number"] {
					width: 100%%;
					padding: 16px 20px;
					border-radius: 16px;
					border: none;
					background-color: var(--input-bg);
					color: var(--text-color);
					font-size: 1.1rem;
					font-weight: 500;
					outline: none;
					transition: var(--transition);
				}

				input[type="number"]:focus {
					background-color: var(--bg-color);
					box-shadow: 0 0 0 2px var(--accent-color);
				}

				button {
					width: 100%%;
					padding: 18px;
					border-radius: 16px;
					border: none;
					background-color: var(--accent-color);
					color: var(--bg-color);
					font-size: 1.1rem;
					font-weight: 600;
					cursor: pointer;
					transition: var(--transition);
				}

				button:hover {
					transform: scale(1.02);
					opacity: 0.9;
				}

				button:active {
					transform: scale(0.98);
				}

				#feedback {
					margin-top: 16px;
					text-align: center;
					font-size: 0.9rem;
					font-weight: 500;
					min-height: 20px;
				}

				.error { color: var(--error); }
				.success { color: var(--success); }

				input::-webkit-outer-spin-button,
				input::-webkit-inner-spin-button {
					-webkit-appearance: none;
					margin: 0;
				}
			</style>
			<script>
				async function fetchState() {
					try {
						let res = await fetch('/state');
						let data = await res.json();
						document.getElementById('status').innerText = data.active ? 'Active' : 'Ended';
						document.getElementById('highestBid').innerText = data.highestBid;
						document.getElementById('winner').innerText = data.winner || 'No bids yet';
						const statusDot = document.querySelector('.status-dot');
						statusDot.style.backgroundColor = data.active ? 'var(--success)' : 'var(--error)';
					} catch (e) {
						console.error('Error fetching state:', e);
					}
				}
				setInterval(fetchState, 1000);
				window.onload = fetchState;

				async function submitBid(e) {
					e.preventDefault();
					let amount = document.getElementById('amount').value;
					let currentBid = parseInt(document.getElementById('highestBid').innerText) || 0;
					let feedback = document.getElementById('feedback');

					feedback.className = '';
					feedback.innerText = 'Submitting...';

					if (parseInt(amount) <= currentBid) {
						feedback.innerText = 'Bid must be higher than current bid ($' + currentBid + ')';
						feedback.className = 'error';
						return false;
					}

					let formData = new URLSearchParams();
					formData.append('amount', amount);

					try {
						let res = await fetch('/bid', {
							method: 'POST',
							body: formData,
							headers: { 'Content-Type': 'application/x-www-form-urlencoded' }
						});

						if (!res.ok) {
							let text = await res.text();
							feedback.innerText = text;
							feedback.className = 'error';
						} else {
							feedback.innerText = await res.text();
							feedback.className = 'success';
							document.getElementById('amount').value = '';
							fetchState();
							setTimeout(() => { feedback.innerText = ''; }, 3000);
						}
					} catch (e) {
						feedback.innerText = 'Network error. Try again.';
						feedback.className = 'error';
					}
					return false;
				}
			</script>
		</head>
		<body>
			<div class="container">
				<header>
					<span class="node-badge">Distributed Node %s</span>
					<h1>Auction</h1>
				</header>

				<div class="card">
					<div class="stat-group">
						<div class="stat-label">CURRENT HIGHEST BID</div>
						<div class="stat-value">$<span id="highestBid">0</span></div>
					</div>
					<div class="stat-group">
						<div class="stat-label">WINNING BIDDER</div>
						<div class="stat-value" id="winner" style="font-size: 1.2rem; color: var(--secondary-text);">None</div>
					</div>
					<div class="status-box">
						<span class="status-dot"></span>
						<span class="status-text" id="status">Active</span>
					</div>
				</div>

				<form onsubmit="return submitBid(event)">
					<div class="input-group">
						<input type="number" id="amount" name="amount" placeholder="Enter bid amount" required autocomplete="off">
					</div>
					<button type="submit">Place Bid</button>
				</form>
				<div id="feedback"></div>
			</div>
		</body>
		</html>
	`, n.ID, n.ID)

	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write([]byte(html))
}

func (n *Node) handleBidRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form request", http.StatusBadRequest)
		return
	}

	amountStr := r.FormValue("amount")
	var amount int
	if _, err := fmt.Sscanf(amountStr, "%d", &amount); err != nil || amount <= 0 {
		http.Error(w, "Invalid bid amount", http.StatusBadRequest)
		return
	}

	coordinatorAddress, isLocalCoordinator := n.getCoordinatorAddress()
	if coordinatorAddress != "" && !isLocalCoordinator {
		var reply CoordinatorBidReply
		err := n.Client.Call(coordinatorAddress, "NodeRPC.SubmitBidToCoordinator", BidArgs{Amount: amount, Bidder: n.ID}, &reply)
		if err != nil {
			http.Error(w, "Leader unavailable; retry shortly", http.StatusServiceUnavailable)
			return
		}
		if !reply.Accepted {
			http.Error(w, reply.Message, http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(reply.Message))
		return
	}

	accepted, message := n.ProposeBid(amount, n.ID)
	if !accepted {
		http.Error(w, message, http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(message))
}

func (n *Node) handleStateRequest(w http.ResponseWriter, r *http.Request) {
	n.ElectionMutex.Lock()
	coordinator := n.Coordinator
	n.ElectionMutex.Unlock()

	n.State.mu.Lock()
	state := map[string]interface{}{
		"highestBid": n.State.HighestBid,
		"winner":     n.State.Winner,
		"active":     n.State.Active,
		"leader":     coordinator,
	}
	n.State.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state)
}

func (n *Node) ProposeBid(amount int, bidder string) (bool, string) {
	txnBid := BidArgs{Amount: amount, Bidder: bidder}
	if !n.canPrepareBid(txnBid) {
		return false, "Bid must be higher than current highest bid"
	}

	n.RA.RequestCS()
	defer n.RA.ReleaseCS()

	if !n.canPrepareBid(txnBid) {
		return false, "Bid became stale during coordination"
	}

	txnID := fmt.Sprintf("%s-%d", n.ID, n.Clock.Tick())
	quorum := (len(n.Peers)+1)/2 + 1
	votes := 1

	n.rememberPendingTxn(txnID, txnBid)

	type voteResult struct{ yes bool }
	voteCh := make(chan voteResult, len(n.Peers))

	for _, peer := range n.Peers {
		go func(p string) {
			var vote PrepareReply
			err := n.Client.Call(p, "NodeRPC.PrepareBid", PrepareArgs{TxnID: txnID, Bid: txnBid, Timestamp: n.Clock.Tick()}, &vote)
			if err != nil {
				voteCh <- voteResult{yes: false}
				return
			}
			voteCh <- voteResult{yes: vote.Vote}
		}(peer)
	}

	pendingResponses := len(n.Peers)
	voteTimer := time.NewTimer(voteWaitTimeout)
	for pendingResponses > 0 {
		if votes >= quorum {
			break
		}
		if votes+pendingResponses < quorum {
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

	commit := votes >= quorum
	n.applyDecision(txnID, commit, txnBid)

	decision := DecisionArgs{TxnID: txnID, Commit: commit, Bid: txnBid, Leader: n.ID}
	for _, peer := range n.Peers {
		go func(p string) {
			var ack bool
			_ = n.Client.Call(p, "NodeRPC.DecideBid", decision, &ack)
		}(peer)
	}

	if !commit {
		log.Printf("[%s] Transaction %s aborted (votes=%d, quorum=%d)\n", n.ID, txnID, votes, quorum)
		return false, fmt.Sprintf("Bid aborted: quorum not reached (%d/%d)", votes, quorum)
	}

	log.Printf("[%s] Transaction %s committed bid=%d bidder=%s (votes=%d, quorum=%d)\n", n.ID, txnID, amount, bidder, votes, quorum)
	return true, "Bid committed by quorum"
}

func (n *Node) canPrepareBid(bid BidArgs) bool {
	n.State.mu.Lock()
	defer n.State.mu.Unlock()
	return n.State.Active && bid.Amount > n.State.HighestBid
}

func (n *Node) rememberPendingTxn(txnID string, bid BidArgs) {
	n.TxnMutex.Lock()
	n.PendingTxns[txnID] = PendingTxn{Bid: bid, PreparedAt: time.Now()}
	n.TxnMutex.Unlock()
}

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

	n.State.mu.Lock()
	if bid.Amount > n.State.HighestBid && n.State.Active {
		n.State.HighestBid = bid.Amount
		n.State.Winner = bid.Bidder
	}
	n.State.mu.Unlock()
}

func (n *Node) abortStalePreparedTxns() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		n.TxnMutex.Lock()
		for txnID, pending := range n.PendingTxns {
			if now.Sub(pending.PreparedAt) > preparedTxnTTL {
				delete(n.PendingTxns, txnID)
				log.Printf("[%s] Auto-aborted stale prepared transaction %s (2PC timeout)\n", n.ID, txnID)
			}
		}
		n.TxnMutex.Unlock()
	}
}

func (n *Node) getCoordinatorAddress() (string, bool) {
	n.ElectionMutex.Lock()
	coordinatorID := n.Coordinator
	n.ElectionMutex.Unlock()

	if coordinatorID == "" || coordinatorID == n.ID {
		return n.Address, true
	}

	rankStr := strings.TrimPrefix(coordinatorID, "Node")
	rank, err := strconv.Atoi(rankStr)
	if err != nil || rank <= 0 {
		return "", false
	}
	coordinatorPort := 8000 + rank
	portSuffix := fmt.Sprintf(":%d", coordinatorPort)
	for _, peer := range n.Peers {
		if strings.HasSuffix(peer, portSuffix) {
			return peer, false
		}
	}
	return fmt.Sprintf("localhost:%d", coordinatorPort), false
}

func (n *Node) periodicStateSync() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		coordinatorAddress, isLocalCoordinator := n.getCoordinatorAddress()
		if isLocalCoordinator || coordinatorAddress == "" {
			continue
		}

		var snapshot StateSnapshot
		err := n.Client.Call(coordinatorAddress, "NodeRPC.GetAuctionState", EmptyArgs{}, &snapshot)
		if err != nil {
			continue
		}

		n.State.mu.Lock()
		if snapshot.HighestBid > n.State.HighestBid {
			n.State.HighestBid = snapshot.HighestBid
			n.State.Winner = snapshot.Winner
		}
		n.State.Active = snapshot.Active
		n.State.mu.Unlock()
	}
}

type BidArgs struct {
	Amount int
	Bidder string
}

type PrepareArgs struct {
	TxnID     string
	Bid       BidArgs
	Timestamp int
}

type PrepareReply struct {
	Vote   bool
	Reason string
}

type DecisionArgs struct {
	TxnID  string
	Commit bool
	Bid    BidArgs
	Leader string
}

type CoordinatorBidReply struct {
	Accepted bool
	Message  string
}

type EmptyArgs struct{}

type StateSnapshot struct {
	HighestBid int
	Winner     string
	Active     bool
}

func (rp *NodeRPC) SubmitBidToCoordinator(args BidArgs, reply *CoordinatorBidReply) error {
	rp.node.ElectionMutex.Lock()
	isCoordinator := rp.node.Coordinator == "" || rp.node.Coordinator == rp.node.ID
	rp.node.ElectionMutex.Unlock()
	if !isCoordinator {
		reply.Accepted = false
		reply.Message = "This node is not the coordinator"
		return nil
	}

	accepted, message := rp.node.ProposeBid(args.Amount, args.Bidder)
	reply.Accepted = accepted
	reply.Message = message
	return nil
}

func (rp *NodeRPC) PrepareBid(args PrepareArgs, reply *PrepareReply) error {
	rp.node.Clock.Update(args.Timestamp)
	if !rp.node.canPrepareBid(args.Bid) {
		reply.Vote = false
		reply.Reason = "bid not higher or auction inactive"
		return nil
	}
	rp.node.rememberPendingTxn(args.TxnID, args.Bid)
	reply.Vote = true
	reply.Reason = "prepared"
	return nil
}

func (rp *NodeRPC) DecideBid(args DecisionArgs, reply *bool) error {
	rp.node.applyDecision(args.TxnID, args.Commit, args.Bid)
	*reply = true
	return nil
}

func (rp *NodeRPC) GetAuctionState(_ EmptyArgs, reply *StateSnapshot) error {
	rp.node.State.mu.Lock()
	reply.HighestBid = rp.node.State.HighestBid
	reply.Winner = rp.node.State.Winner
	reply.Active = rp.node.State.Active
	rp.node.State.mu.Unlock()
	return nil
}

// RPC Methods
func (rp *NodeRPC) HandleRARequest(args RAMessage, reply *bool) error {
	*reply = rp.node.RA.ReceiveRequest(args)
	return nil
}

func (rp *NodeRPC) HandleRADeferredReply(args RAMessage, reply *bool) error {
	rp.node.RA.HandleRAReply()
	*reply = true
	return nil
}

// Legacy propagation handler retained for backward compatibility.
func (rp *NodeRPC) HandleBid(args BidArgs, reply *bool) error {
	rp.node.State.mu.Lock()
	if args.Amount > rp.node.State.HighestBid && rp.node.State.Active {
		rp.node.State.HighestBid = args.Amount
		rp.node.State.Winner = args.Bidder
		log.Printf("[%s] Legacy bid sync: %d by %s\n", rp.node.ID, args.Amount, args.Bidder)
	}
	rp.node.State.mu.Unlock()
	*reply = true
	return nil
}
