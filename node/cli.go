package node

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// StartCLI starts a command-line interface for the node.
func (n *Node) StartCLI() {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Printf("\n[%s] CLI Active. Type 'help' for commands.\n", n.ID)
	fmt.Print("> ")

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			fmt.Print("> ")
			continue
		}

		parts := strings.Fields(line)
		cmd := strings.ToLower(parts[0])

		switch cmd {
		case "help":
			n.printHelp()
		case "status":
			n.printStatus()
		case "queue":
			n.printQueue()
		case "peers":
			n.printPeers()
		case "bid":
			n.handleCLIBid(parts[1:])
		case "add":
			n.handleCLIAdd(scanner) // Pass scanner for interactive input
		case "start":
			n.handleCLIControl("start")
		case "stop":
			n.handleCLIControl("stop")
		case "restart":
			n.handleCLIControl("restart")
		case "exit", "quit":
			fmt.Println("Exiting process...")
			os.Exit(0)
		default:
			fmt.Printf("Unknown command: %s. Type 'help' for usage.\n", cmd)
		}
		fmt.Print("> ")
	}
}

func (n *Node) printHelp() {
	fmt.Println("\nAvailable Commands:")
	fmt.Println("  status                          - Show current auction status")
	fmt.Println("  queue                           - Show upcoming items in the queue")
	fmt.Println("  bid <amount> [name]             - Place a bid on the current item")
	fmt.Println("  peers                           - List all peer addresses")
	fmt.Println("  add                             - Add a new item to the queue (Interactive, Coordinator only)")
	fmt.Println("  start                           - Start the auction (Coordinator only)")
	fmt.Println("  stop                            - Stop the auction (Coordinator only)")
	fmt.Println("  restart                         - Restart auction from default items (Coordinator only)")
	fmt.Println("  help                            - Show this help message")
	fmt.Println("  exit/quit                       - Terminate this node process")
}

func (n *Node) printStatus() {
	snap := n.buildQueueSnapshot()
	fmt.Println("\n--- Auction Status ---")
	statusStr := "Inactive"
	if snap.Active {
		statusStr = "Active"
	}
	fmt.Printf("Status:         %s\n", statusStr)

	if snap.CurrentItem != nil {
		fmt.Printf("Current Item:   %s\n", snap.CurrentItem.Name)
		fmt.Printf("Description:    %s\n", snap.CurrentItem.Description)
		fmt.Printf("Highest Bid:    $%d (by %s)\n", snap.CurrentHighestBid, snap.CurrentWinner)

		rem := snap.DeadlineUnix - time.Now().Unix()
		if rem < 0 {
			rem = 0
		}
		fmt.Printf("Time Remaining: %ds\n", rem)
	} else {
		fmt.Println("Current Item:   None")
	}
	fmt.Printf("Items in Queue: %d\n", snap.QueueLen)
	fmt.Printf("Items Sold:     %d\n", len(snap.Results))
	fmt.Printf("Is Leader:      %v\n", snap.IsCoordinator)
	fmt.Println("----------------------")
}

func (n *Node) printQueue() {
	snap := n.buildQueueSnapshot()
	fmt.Println("\n--- Up Next ---")
	if len(snap.RemainingItems) == 0 {
		fmt.Println("No items in the queue.")
	} else {
		for i, it := range snap.RemainingItems {
			fmt.Printf("[%d] %s ($%d, %ds)\n", i+1, it.Name, it.StartingPrice, it.DurationSec)
			fmt.Printf("    %s\n", it.Description)
		}
	}
	fmt.Println("----------------")
}

func (n *Node) printPeers() {
	fmt.Println("\n--- Peer Nodes ---")
	if len(n.Peers) == 0 {
		fmt.Println("No peers configured.")
	} else {
		for i, p := range n.Peers {
			fmt.Printf("[%d] %s\n", i+1, p)
		}
	}
	fmt.Println("------------------")
}

func (n *Node) handleCLIBid(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: bid <amount> [name]")
		return
	}

	amount, err := strconv.Atoi(args[0])
	if err != nil || amount <= 0 {
		fmt.Println("Invalid bid amount.")
		return
	}

	bidder := n.ID
	if len(args) >= 2 {
		bidder = strings.Join(args[1:], " ")
	}

	coordinatorAddress, isLocalCoordinator := n.getCoordinatorAddress()
	if !isLocalCoordinator {
		if coordinatorAddress == "" {
			fmt.Println("Election in progress, please wait...")
			return
		}
		var reply CoordinatorBidReply
		err := n.callPeer(coordinatorAddress, "NodeRPC.SubmitBidToCoordinator",
			BidArgs{Amount: amount, Bidder: bidder}, &reply)
		if err != nil {
			fmt.Printf("Error forwarding bid to coordinator: %v\n", err)
			return
		}
		if !reply.Accepted {
			fmt.Printf("Bid rejected: %s\n", reply.Message)
		} else {
			fmt.Printf("Bid accepted: %s\n", reply.Message)
		}
		return
	}

	accepted, message := n.ProposeBid(amount, bidder)
	if !accepted {
		fmt.Printf("Bid rejected: %s\n", message)
	} else {
		fmt.Printf("Bid accepted: %s\n", message)
	}
}

func (n *Node) handleCLIAdd(scanner *bufio.Scanner) {
	fmt.Println("\n--- Add New Item ---")

	fmt.Print("Name: ")
	if !scanner.Scan() {
		return
	}
	name := strings.TrimSpace(scanner.Text())

	fmt.Print("Description: ")
	if !scanner.Scan() {
		return
	}
	desc := strings.TrimSpace(scanner.Text())

	fmt.Print("Starting Price ($): ")
	if !scanner.Scan() {
		return
	}
	priceStr := strings.TrimSpace(scanner.Text())
	price, err1 := strconv.Atoi(priceStr)

	fmt.Print("Duration (seconds): ")
	if !scanner.Scan() {
		return
	}
	durStr := strings.TrimSpace(scanner.Text())
	dur, err2 := strconv.Atoi(durStr)

	if err1 != nil || err2 != nil || price <= 0 || dur <= 0 {
		fmt.Println("Error: Invalid price or duration. Item not added.")
		return
	}

	coordinatorAddress, isLocalCoordinator := n.getCoordinatorAddress()
	if !isLocalCoordinator {
		if coordinatorAddress == "" {
			fmt.Println("Election in progress...")
			return
		}
		var reply CoordinatorActionReply
		err := n.callPeer(coordinatorAddress, "NodeRPC.SubmitAddItemToCoordinator",
			AddItemArgs{Name: name, Description: desc, StartingPrice: price, DurationSec: dur}, &reply)
		if err != nil {
			fmt.Printf("Error forwarding to coordinator: %v\n", err)
			return
		}
		fmt.Println(reply.Message)
		return
	}

	accepted, message := n.addItemAndBroadcast(name, desc, price, dur)
	fmt.Printf("[%v] %s\n", accepted, message)
}

func (n *Node) handleCLIControl(action string) {
	coordinatorAddress, isLocalCoordinator := n.getCoordinatorAddress()
	if !isLocalCoordinator {
		if coordinatorAddress == "" {
			fmt.Println("Election in progress...")
			return
		}
		var reply CoordinatorActionReply
		err := n.callPeer(coordinatorAddress, "NodeRPC.SubmitAuctionControlToCoordinator",
			AuctionControlArgs{Action: action}, &reply)
		if err != nil {
			fmt.Printf("Error forwarding to coordinator: %v\n", err)
			return
		}
		fmt.Println(reply.Message)
		return
	}

	var accepted bool
	var message string
	switch action {
	case "start":
		accepted, message = n.startAuctionAndBroadcast()
	case "stop":
		accepted, message = n.stopAuctionAndBroadcast()
	case "restart":
		accepted, message = n.restartAuctionAndBroadcast()
	}
	fmt.Printf("[%v] %s\n", accepted, message)
}
