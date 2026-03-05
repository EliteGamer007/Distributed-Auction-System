package main

import (
	"auction_node/node"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

func main() {
	id := flag.String("id", "", "Node ID")
	host := flag.String("host", "0.0.0.0", "Host/IP to bind on (use 0.0.0.0 for LAN)")
	port := flag.String("port", "", "Port to listen on")
	peersList := flag.String("peers", "", "Comma separated list of peer addresses (e.g. localhost:8081,localhost:8082)")
	launchMode := flag.String("launch", "", "Launch mode: 'local' (4 nodes) or 'lan' (current node in terminal)")
	logToFile := flag.Bool("log-to-file", false, "Redirect logs to node<ID>.log instead of stdout")
	flag.Parse()

	if *launchMode != "" {
		spawnTerminals(*launchMode, *id)
		return
	}

	if *id == "" || *port == "" {
		fmt.Println("Usage: main --id <node_id> --port <port> --peers <peer_addresses>")
		fmt.Println("       main --launch local")
		fmt.Println("       main --launch lan --id Node1")
		os.Exit(1)
	}

	if *logToFile {
		logFile, err := os.OpenFile(fmt.Sprintf("%s.log", strings.ToLower(*id)), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err == nil {
			log.SetOutput(logFile)
		}
	}

	peers := []string{}
	// ... rest of main ...
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

func spawnTerminals(mode, nodeID string) {
	// List of common terminal emulators to try
	terminals := []string{
		"x-terminal-emulator",
		"gnome-terminal",
		"konsole",
		"xfce4-terminal",
		"terminator",
		"xterm",
		"uxterm",
		"kitty",
		"alacritty",
		"mate-terminal",
	}

	launch := func(cmd string) {
		// Use a shell script that checks for the first available terminal
		var builder strings.Builder
		builder.WriteString("run_term() { ")
		for i, term := range terminals {
			var termCmd string
			switch term {
			case "gnome-terminal", "mate-terminal", "xfce4-terminal":
				termCmd = fmt.Sprintf("%s -- bash -c \"%s; exec bash\"", term, cmd)
			case "konsole":
				termCmd = fmt.Sprintf("%s -e bash -c \"%s; exec bash\"", term, cmd)
			default:
				termCmd = fmt.Sprintf("%s -e \"bash -c '%s; exec bash'\"", term, cmd)
			}
			if i == 0 {
				builder.WriteString(fmt.Sprintf("if command -v %s >/dev/null 2>&1; then %s & return 0; fi; ", term, termCmd))
			} else {
				builder.WriteString(fmt.Sprintf("if command -v %s >/dev/null 2>&1; then %s & return 0; fi; ", term, termCmd))
			}
		}
		builder.WriteString("echo 'Error: No terminal emulator found'; return 1; }; run_term")

		execCmd := builder.String()
		os.StartProcess("/bin/bash", []string{"bash", "-c", execCmd}, &os.ProcAttr{Files: []*os.File{os.Stdin, os.Stdout, os.Stderr}})
	}

	if mode == "local" {
		fmt.Println("Launching 4 nodes in separate terminals...")
		peers := "localhost:8001,localhost:8002,localhost:8003,localhost:8004"
		for i := 1; i <= 4; i++ {
			port := 8000 + i
			id := fmt.Sprintf("Node%d", i)
			cmd := fmt.Sprintf("./auction_node --id %s --port %d --peers %s --log-to-file", id, port, peers)
			launch(cmd)
		}
	} else if mode == "lan" {
		if nodeID == "" {
			fmt.Println("Error: --id is required for LAN mode (e.g. --id Node1)")
			return
		}
		fmt.Printf("Launching %s in a new terminal...\n", nodeID)
		rankStr := strings.TrimPrefix(nodeID, "Node")
		// The script already has redirection logic, but we can ensure it stays interactive
		cmd := fmt.Sprintf("./start_lan_node.sh %s", rankStr)
		launch(cmd)
	} else {
		fmt.Println("Unknown launch mode:", mode)
	}
}
