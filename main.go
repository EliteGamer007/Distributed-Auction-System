package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"distributed-auction/node"
)

func main() {
	id := flag.String("id", "", "Node ID")
	host := flag.String("host", "0.0.0.0", "Host/IP to bind on (use 0.0.0.0 for LAN)")
	port := flag.String("port", "", "Port to listen on")
	peersList := flag.String("peers", "", "Comma separated list of peer addresses (e.g. localhost:8081,localhost:8082)")
	flag.Parse()

	if *id == "" || *port == "" {
		fmt.Println("Usage: main --id <node_id> --port <port> --peers <peer_addresses>")
		os.Exit(1)
	}

	peers := []string{}
	if *peersList != "" {
		peers = strings.Split(*peersList, ",")
	}

	address := fmt.Sprintf("%s:%s", *host, *port)

	// Derive rank from node ID (e.g. Node1 -> 1)
	rankStr := strings.TrimPrefix(*id, "Node")
	rank, err := strconv.Atoi(rankStr)
	if err != nil {
		fmt.Printf("Error: Node ID must be in format 'Node<number>', got '%s'\n", *id)
		os.Exit(1)
	}

	n := node.NewNode(*id, address, peers, rank)
	n.Start()

	// Start bully leader monitoring
	go n.MonitorLeader()

	// Block forever
	select {}
}
