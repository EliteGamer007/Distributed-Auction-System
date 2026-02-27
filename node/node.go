package node

// node.go — Node struct definition, constructor, and HTTP server startup.

import (
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

// Node is the main distributed auction node.
type Node struct {
	ID            string
	Address       string
	Peers         []string
	Queue         *ItemQueueState
	Clock         *LamportClock
	RA            *RAManager
	Client        *RPCClient
	Rank          int
	Coordinator   string
	ElectionMutex sync.Mutex
	LeaderChan    chan bool
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

	items := defaultItems()
	queue := &ItemQueueState{
		Queue:  items[1:],
		Active: true,
	}
	first := items[0]
	queue.CurrentItem = &first
	queue.CurrentHighestBid = first.StartingPrice - 1

	return &Node{
		ID:          id,
		Address:     address,
		Peers:       peers,
		Queue:       queue,
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

// getCoordinatorAddress resolves the coordinator's TCP address.
// Returns (address, isLocal): isLocal=true means this node IS the coordinator.
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
