package node

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"sync"
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
		ID:         id,
		Address:    address,
		Peers:      peers,
		State:      state,
		Clock:      clock,
		RA:         ra,
		Client:     client,
		Rank:       rank,
		LeaderChan: make(chan bool),
	}
}

func (n *Node) Start() {
	rpcServer := &NodeRPC{node: n}
	server := rpc.NewServer()
	server.Register(rpcServer)

	listener, err := net.Listen("tcp", n.Address)
	if err != nil {
		log.Fatalf("Listen error: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle(rpc.DefaultRPCPath, server)

	// Minimal UI endpoints
	mux.HandleFunc("/", n.handleUI)
	mux.HandleFunc("/bid", n.handleBidRequest)
	mux.HandleFunc("/state", n.handleStateRequest)

	go http.Serve(listener, mux)
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

				/* Chrome, Safari, Edge, Opera */
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
						console.error("Error fetching state:", e);
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
							feedback.innerText = 'Success! Bid placed.';
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
	w.Write([]byte(html))
}

func (n *Node) handleBidRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.ParseForm()
	amountStr := r.FormValue("amount")
	var amount int
	fmt.Sscanf(amountStr, "%d", &amount)

	n.State.mu.Lock()
	if amount <= n.State.HighestBid || !n.State.Active {
		n.State.mu.Unlock()
		http.Error(w, "Bid must be higher than current highest bid", http.StatusBadRequest)
		return
	}
	n.State.mu.Unlock()

	go n.PlaceBid(amount)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Bid Accepted"))
}

func (n *Node) handleStateRequest(w http.ResponseWriter, r *http.Request) {
	n.State.mu.Lock()
	state := map[string]interface{}{
		"highestBid": n.State.HighestBid,
		"winner":     n.State.Winner,
		"active":     n.State.Active,
	}
	n.State.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

func (n *Node) PlaceBid(amount int) {
	n.State.mu.Lock()
	if amount <= n.State.HighestBid || !n.State.Active {
		n.State.mu.Unlock()
		return
	}
	n.State.mu.Unlock()

	// Mutual Exclusion to place bid
	n.RA.RequestCS()
	defer n.RA.ReleaseCS()

	// Double check
	n.State.mu.Lock()
	if amount <= n.State.HighestBid || !n.State.Active {
		n.State.mu.Unlock()
		return
	}
	n.State.HighestBid = amount
	n.State.Winner = n.ID
	n.State.mu.Unlock()

	log.Printf("[%s] Placed new highest bid %d\n", n.ID, amount)

	// Propagate to peers
	for _, peer := range n.RA.Peers {
		go func(p string) {
			var reply bool
			n.Client.Call(p, "NodeRPC.HandleBid", BidArgs{Amount: amount, Bidder: n.ID}, &reply)
		}(peer)
	}
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

type BidArgs struct {
	Amount int
	Bidder string
}

func (rp *NodeRPC) HandleBid(args BidArgs, reply *bool) error {
	rp.node.State.mu.Lock()
	defer rp.node.State.mu.Unlock()

	if args.Amount > rp.node.State.HighestBid {
		rp.node.State.HighestBid = args.Amount
		rp.node.State.Winner = args.Bidder
		log.Printf("[%s] Received new highest bid: %d by %s\n", rp.node.ID, args.Amount, args.Bidder)
	}
	*reply = true
	return nil
}
