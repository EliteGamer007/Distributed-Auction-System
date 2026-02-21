package node

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
)

type Node struct {
	ID      string
	Address string
	Peers   []string
	State   *AuctionState
	Clock   *LamportClock
	RA      *RAManager
	Client  *RPCClient
}

type NodeRPC struct {
	node *Node
}

func NewNode(id, address string, peers []string) *Node {
	clock := &LamportClock{}
	client := &RPCClient{}
	ra := NewRAManager(id, peers, clock, client)
	state := &AuctionState{Active: true}

	return &Node{
		ID:      id,
		Address: address,
		Peers:   peers,
		State:   state,
		Clock:   clock,
		RA:      ra,
		Client:  client,
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
		<html>
		<head>
			<title>Node %s Auction</title>
			<script>
				async function fetchState() {
					try {
						let res = await fetch('/state');
						let data = await res.json();
						document.getElementById('status').innerText = data.active;
						document.getElementById('highestBid').innerText = data.highestBid;
						document.getElementById('winner').innerText = data.winner;
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
					let errorEl = document.getElementById('error');
					let successEl = document.getElementById('success');
					
					errorEl.innerText = '';
					successEl.innerText = '';

					if (parseInt(amount) <= currentBid) {
						errorEl.innerText = 'Error: Bid must be higher than the current winning bid of $' + currentBid;
						return false;
					}

					let formData = new URLSearchParams();
					formData.append('amount', amount);

					let res = await fetch('/bid', {
						method: 'POST',
						body: formData,
						headers: { 'Content-Type': 'application/x-www-form-urlencoded' }
					});

					if (!res.ok) {
						let text = await res.text();
						errorEl.innerText = 'Error: ' + text;
					} else {
						successEl.innerText = 'Bid request submitted.';
						document.getElementById('amount').value = '';
						fetchState();
					}
					return false;
				}
			</script>
		</head>
		<body style="font-family: sans-serif; padding: 20px; max-width: 600px; margin: 0 auto;">
			<div style="text-align: center; margin-bottom: 20px;">
				<h1 style="color: #333; margin-bottom: 10px;">Distributed Auction System</h1>
				<div style="display: inline-block; background-color: #007BFF; color: white; padding: 5px 15px; border-radius: 20px; font-weight: bold; font-size: 1.2em;">
					Node %s
				</div>
				<hr style="margin-top: 20px; border: 0; border-top: 2px solid #eee;" />
			</div>
			<div style="background: #f9f9f9; padding: 15px; border-radius: 8px; margin-bottom: 20px;">
				<p><strong>Status:</strong> <span id="status"></span></p>
				<p><strong>Highest Bid:</strong> $<span id="highestBid"></span></p>
				<p><strong>Winner:</strong> <span id="winner"></span></p>
			</div>
			
			<form onsubmit="return submitBid(event)">
				<label style="font-weight: bold; display: block; margin-bottom: 5px;">Your Bid Amount:</label>
				<input type="number" id="amount" name="amount" required style="padding: 8px; border-radius: 4px; border: 1px solid #ccc; width: 100%%; box-sizing: border-box; margin-bottom: 10px;">
				<button type="submit" style="background-color: #28a745; color: white; border: none; padding: 10px 20px; border-radius: 5px; cursor: pointer; font-size: 1em; width: 100%%;">Place Bid</button>
			</form>
			<p id="error" style="color: red; font-weight: bold;"></p>
			<p id="success" style="color: green; font-weight: bold;"></p>
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
