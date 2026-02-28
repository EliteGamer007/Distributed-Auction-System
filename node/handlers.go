package node

// handlers.go — HTTP request handlers for /bid, /state, and /checkpoint endpoints.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

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
	bidder := r.FormValue("bidder")
	if bidder == "" {
		bidder = n.ID
	}

	var amount int
	if _, err := fmt.Sscanf(amountStr, "%d", &amount); err != nil || amount <= 0 {
		http.Error(w, "Invalid bid amount", http.StatusBadRequest)
		return
	}

	coordinatorAddress, isLocalCoordinator := n.getCoordinatorAddress()
	if coordinatorAddress != "" && !isLocalCoordinator {
		// Forward to coordinator
		var reply CoordinatorBidReply
		err := n.Client.Call(coordinatorAddress, "NodeRPC.SubmitBidToCoordinator",
			BidArgs{Amount: amount, Bidder: bidder}, &reply)
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

	// This node is the coordinator — run 2PC directly
	accepted, message := n.ProposeBid(amount, bidder)
	if !accepted {
		http.Error(w, message, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(message))
}

func (n *Node) handleStateRequest(w http.ResponseWriter, r *http.Request) {
	snap := n.buildQueueSnapshot()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(snap)
}

func (n *Node) handleAddItemRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := ""
	description := ""
	startingPrice := 0
	durationSec := 0

	if strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		var req struct {
			Name          string `json:"name"`
			Description   string `json:"description"`
			StartingPrice int    `json:"startingPrice"`
			DurationSec   int    `json:"durationSec"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "Invalid JSON request", http.StatusBadRequest)
			return
		}
		name = req.Name
		description = req.Description
		startingPrice = req.StartingPrice
		durationSec = req.DurationSec
	} else {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form request", http.StatusBadRequest)
			return
		}
		name = r.FormValue("name")
		description = r.FormValue("description")
		if _, err := fmt.Sscanf(r.FormValue("startingPrice"), "%d", &startingPrice); err != nil {
			http.Error(w, "Invalid starting price", http.StatusBadRequest)
			return
		}
		if _, err := fmt.Sscanf(r.FormValue("durationSec"), "%d", &durationSec); err != nil {
			http.Error(w, "Invalid duration", http.StatusBadRequest)
			return
		}
	}

	coordinatorAddress, isLocalCoordinator := n.getCoordinatorAddress()
	if coordinatorAddress != "" && !isLocalCoordinator {
		var reply CoordinatorActionReply
		err := n.Client.Call(coordinatorAddress, "NodeRPC.SubmitAddItemToCoordinator",
			AddItemArgs{Name: name, Description: description, StartingPrice: startingPrice, DurationSec: durationSec}, &reply)
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

	accepted, message := n.addItemAndBroadcast(name, description, startingPrice, durationSec)
	if !accepted {
		http.Error(w, message, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(message))
}

func (n *Node) handleAuctionControlRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	action := ""
	if strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		var req struct {
			Action string `json:"action"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "Invalid JSON request", http.StatusBadRequest)
			return
		}
		action = req.Action
	} else {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form request", http.StatusBadRequest)
			return
		}
		action = r.FormValue("action")
	}

	if action != "start" && action != "restart" {
		http.Error(w, "Unsupported action", http.StatusBadRequest)
		return
	}

	coordinatorAddress, isLocalCoordinator := n.getCoordinatorAddress()
	if coordinatorAddress != "" && !isLocalCoordinator {
		var reply CoordinatorActionReply
		err := n.Client.Call(coordinatorAddress, "NodeRPC.SubmitAuctionControlToCoordinator",
			AuctionControlArgs{Action: action}, &reply)
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

	var accepted bool
	var message string
	if action == "start" {
		accepted, message = n.startAuctionAndBroadcast()
	} else {
		accepted, message = n.restartAuctionAndBroadcast()
	}

	if !accepted {
		http.Error(w, message, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(message))
}

// handleCheckpointRequest serves the raw checkpoint file for this node.
func (n *Node) handleCheckpointRequest(w http.ResponseWriter, r *http.Request) {
	b, err := os.ReadFile(checkpointPath(n.ID))
	if os.IsNotExist(err) {
		http.Error(w, "No checkpoint yet", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Could not read checkpoint", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}
