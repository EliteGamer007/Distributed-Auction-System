package main

import (
	"auction_node/node"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	id := flag.String("id", "", "Node ID")
	host := flag.String("host", "0.0.0.0", "Host/IP to bind on (use 0.0.0.0 for LAN)")
	port := flag.String("port", "", "Port to listen on")
	peersList := flag.String("peers", "", "Comma separated list of peer addresses (e.g. localhost:8081,localhost:8082)")
	launchMode := flag.String("launch", "", "Launch mode: 'local' (4 nodes + monitor) or 'lan' (current node in terminal)")
	logToFile := flag.Bool("log-to-file", false, "Redirect logs to node<ID>.log instead of stdout")
	isMonitor := flag.Bool("monitor", false, "Run as an auction monitor dashboard")
	isLogViewer := flag.Bool("log-viewer", false, "Run as a combined log viewer (tail -f node*.log)")
	flag.Parse()

	if *isMonitor {
		node.RunMonitor()
		return
	}

	if *isLogViewer {
		runLogViewer()
		return
	}

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
		for _, term := range terminals {
			var termCmd string
			switch term {
			case "gnome-terminal", "mate-terminal", "xfce4-terminal":
				termCmd = fmt.Sprintf("%s -- bash -c '%s; exec bash'", term, cmd)
			case "konsole":
				termCmd = fmt.Sprintf("%s -e bash -c '%s; exec bash'", term, cmd)
			default:
				termCmd = fmt.Sprintf("%s -e \"bash -c '%s; exec bash'\"", term, cmd)
			}
			builder.WriteString(fmt.Sprintf("if command -v %s >/dev/null 2>&1; then %s & return 0; fi; ", term, termCmd))
		}
		builder.WriteString("echo 'Error: No terminal emulator found'; return 1; }; run_term")

		execCmd := builder.String()
		os.StartProcess("/bin/bash", []string{"bash", "-c", execCmd}, &os.ProcAttr{Files: []*os.File{os.Stdin, os.Stdout, os.Stderr}})
	}

	if mode == "local" {
		fmt.Println("Launching 4 nodes + 1 monitor + 1 log viewer in separate terminals...")
		peers := "localhost:8001,localhost:8002,localhost:8003,localhost:8004"
		for i := 1; i <= 4; i++ {
			port := 8000 + i
			id := fmt.Sprintf("Node%d", i)
			cmd := fmt.Sprintf("./auction_node --id %s --port %d --peers %s --log-to-file", id, port, peers)
			launch(cmd)
		}
		// Launch 5th Monitor Terminal
		time.Sleep(500 * time.Millisecond)
		launch("./auction_node --monitor")
		// Launch 6th Log Viewer Terminal
		time.Sleep(500 * time.Millisecond)
		launch("./auction_node --log-viewer")
	} else if mode == "node" {
		if nodeID == "" {
			fmt.Println("Error: --id is required (e.g. Node1, Node2, ...)")
			return
		}
		rankStr := strings.TrimPrefix(nodeID, "Node")
		rank, err := strconv.Atoi(rankStr)
		if err != nil || rank < 1 || rank > 4 {
			fmt.Println("Error: Invalid Node ID. Use Node1, Node2, Node3, or Node4.")
			return
		}
		port := 8000 + rank
		peers := "localhost:8001,localhost:8002,localhost:8003,localhost:8004"
		fmt.Printf("Restarting %s in a new terminal...\n", nodeID)
		cmd := fmt.Sprintf("./auction_node --id %s --port %d --peers %s --log-to-file", nodeID, port, peers)
		launch(cmd)
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

func runLogViewer() {
	fmt.Println("--- Combined Node Logs (tail -f node*.log) ---")
	// Use tail -f on all potential node log files
	// We use the shell's tail command for simplicity and robust tailing
	cmd := "tail -f node1.log node2.log node3.log node4.log 2>/dev/null || tail -f node1.log"
	os.StartProcess("/bin/bash", []string{"bash", "-c", cmd}, &os.ProcAttr{Files: []*os.File{os.Stdin, os.Stdout, os.Stderr}})
	// Block forever
	select {}
}
