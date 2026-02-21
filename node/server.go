package node

import (
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

	go http.Serve(listener, mux)
	log.Printf("Node %s listening on %s (UI at http://%s)\n", n.ID, n.Address, n.Address)
}

func (n *Node) handleUI(w http.ResponseWriter, r *http.Request) {
	n.State.mu.Lock()
	highestBid := n.State.HighestBid
	winner := n.State.Winner
	active := n.State.Active
	n.State.mu.Unlock()

	html := fmt.Sprintf(`
		<html>
		<head><title>Node %s Auction</title></head>
		<body style="font-family: sans-serif; padding: 20px;">
			<h2>Distributed Auction - Node %s</h2>
			<p>Status: %t</p>
			<p>Highest Bid: $%d</p>
			<p>Winner: %s</p>
			<hr/>
			<form action="/bid" method="POST">
				<label>Your Bid Amount:</label>
				<input type="number" name="amount" required>
				<button type="submit">Place Bid</button>
			</form>
		</body>
		</html>
	`, n.ID, n.ID, active, highestBid, winner)
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

	go n.PlaceBid(amount)

	http.Redirect(w, r, "/", http.StatusSeeOther)
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
