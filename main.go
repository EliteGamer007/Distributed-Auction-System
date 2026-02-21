package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"distributed-auction/node"
)

func main() {
	id := flag.String("id", "", "Node ID")
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

	address := fmt.Sprintf("localhost:%s", *port)
	n := node.NewNode(*id, address, peers)
	n.Start()

	// Block forever
	select {}
}
