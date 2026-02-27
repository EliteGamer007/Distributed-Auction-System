package node

// handlers.go — HTTP request handlers for /bid and /state endpoints.

import (
	"encoding/json"
	"fmt"
	"net/http"
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
