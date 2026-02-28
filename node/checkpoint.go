package node

// checkpoint.go — Coordinated checkpointing.
//
// Algorithm (coordinator-initiated):
//  1. Coordinator snapshots its own state to disk.
//  2. Coordinator broadcasts TakeCheckpoint RPC to all followers.
//  3. Each follower snapshots its own state and ACKs.
//  4. Coordinator logs completion with the global Lamport timestamp.
//
// On startup, every node reads its checkpoint file and restores state
// before falling back to the default item seed list.

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

const (
	checkpointDir        = "checkpoints"
	checkpointInterval   = 30 * time.Second
	checkpointAckTimeout = 5 * time.Second
)

// CheckpointData is the full serialisable state of a node, written to disk.
type CheckpointData struct {
	NodeID            string        `json:"nodeId"`
	LamportTime       int           `json:"lamportTime"`
	CurrentItem       *AuctionItem  `json:"currentItem"`
	RemainingQueue    []AuctionItem `json:"remainingQueue"`
	Results           []ItemResult  `json:"results"`
	CurrentHighestBid int           `json:"currentHighestBid"`
	CurrentWinner     string        `json:"currentWinner"`
	DeadlineUnix      int64         `json:"deadlineUnix"`
	Active            bool          `json:"active"`
	CheckpointTime    int64         `json:"checkpointTime"` // wall-clock Unix
	LamportStamp      int           `json:"lamportStamp"`   // Lamport time at checkpoint
}

// checkpointPath returns the file path for a node's checkpoint.
func checkpointPath(nodeID string) string {
	return filepath.Join(checkpointDir, fmt.Sprintf("checkpoint_%s.json", nodeID))
}

// saveCheckpoint writes data to checkpoints/<NodeID>.json atomically.
func saveCheckpoint(data CheckpointData) error {
	if err := os.MkdirAll(checkpointDir, 0o755); err != nil {
		return fmt.Errorf("mkdir checkpoints: %w", err)
	}
	path := checkpointPath(data.NodeID)
	tmp := path + ".tmp"

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal checkpoint: %w", err)
	}
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return fmt.Errorf("write checkpoint tmp: %w", err)
	}
	// Atomic rename — a partial write never corrupts the last good checkpoint.
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename checkpoint: %w", err)
	}
	return nil
}

// loadCheckpoint reads checkpoints/<nodeID>.json.
// Returns (nil, nil) if no checkpoint exists yet.
func loadCheckpoint(nodeID string) (*CheckpointData, error) {
	path := checkpointPath(nodeID)
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read checkpoint: %w", err)
	}
	var data CheckpointData
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, fmt.Errorf("parse checkpoint: %w", err)
	}
	return &data, nil
}

// takeLocalCheckpoint snapshots this node's current state and saves it to disk.
func (n *Node) takeLocalCheckpoint() error {
	n.Queue.mu.Lock()
	data := CheckpointData{
		NodeID:            n.ID,
		LamportTime:       n.Clock.Get(),
		LamportStamp:      n.Clock.Get(),
		CurrentHighestBid: n.Queue.CurrentHighestBid,
		CurrentWinner:     n.Queue.CurrentWinner,
		DeadlineUnix:      n.Queue.DeadlineUnix,
		Active:            n.Queue.Active,
		Results:           append([]ItemResult(nil), n.Queue.Results...),
		RemainingQueue:    append([]AuctionItem(nil), n.Queue.Queue...),
		CheckpointTime:    time.Now().Unix(),
	}
	if n.Queue.CurrentItem != nil {
		item := *n.Queue.CurrentItem
		data.CurrentItem = &item
	}
	n.Queue.mu.Unlock()

	if err := saveCheckpoint(data); err != nil {
		return err
	}
	log.Printf("[%s] 📸 Checkpoint saved (lamport=%d, item=%v, results=%d)\n",
		n.ID, data.LamportStamp, itemName(data.CurrentItem), len(data.Results))
	return nil
}

// initiateGlobalCheckpoint is called by the coordinator to checkpoint all nodes.
func (n *Node) initiateGlobalCheckpoint() {
	n.ElectionMutex.Lock()
	isCoordinator := n.Coordinator == "" || n.Coordinator == n.ID
	n.ElectionMutex.Unlock()
	if !isCoordinator {
		return
	}

	lamport := n.Clock.Tick()
	log.Printf("[%s] 🟢 Initiating global checkpoint at Lamport=%d\n", n.ID, lamport)

	// Step 1: save coordinator's own state.
	if err := n.takeLocalCheckpoint(); err != nil {
		log.Printf("[%s] ⚠️  Local checkpoint failed: %v\n", n.ID, err)
	}

	// Step 2: ask all followers to checkpoint simultaneously.
	type ackResult struct {
		peer    string
		lamport int
		err     error
	}
	ackCh := make(chan ackResult, len(n.Peers))
	args := TakeCheckpointArgs{InitiatorID: n.ID, LamportTime: lamport}

	for _, peer := range n.Peers {
		go func(p string) {
			var reply TakeCheckpointReply
			err := n.Client.Call(p, "NodeRPC.TakeCheckpoint", args, &reply)
			if err == nil && !reply.OK {
				err = fmt.Errorf("%s", reply.Error)
			}
			ackCh <- ackResult{peer: p, lamport: reply.LamportStamp, err: err}
		}(peer)
	}

	// Step 3: collect ACKs with timeout.
	timer := time.NewTimer(checkpointAckTimeout)
	defer timer.Stop()
	acks := 0
	for acks < len(n.Peers) {
		select {
		case res := <-ackCh:
			if res.err != nil {
				log.Printf("[%s] ⚠️  Checkpoint NACK from %s: %v\n", n.ID, res.peer, res.err)
			} else {
				acks++
				log.Printf("[%s] ✅ Checkpoint ACK from %s (lamport=%d)\n", n.ID, res.peer, res.lamport)
			}
		case <-timer.C:
			log.Printf("[%s] ⚠️  Checkpoint timed out (%d/%d ACKs)\n", n.ID, acks, len(n.Peers))
			return
		}
	}
	log.Printf("[%s] 🏁 Global checkpoint complete — %d nodes saved (lamport=%d)\n",
		n.ID, len(n.Peers)+1, lamport)
}

// runPeriodicCheckpointing triggers a global checkpoint every 30s (coordinator only).
func (n *Node) runPeriodicCheckpointing() {
	ticker := time.NewTicker(checkpointInterval)
	defer ticker.Stop()
	for range ticker.C {
		n.ElectionMutex.Lock()
		isCoordinator := n.Coordinator == "" || n.Coordinator == n.ID
		n.ElectionMutex.Unlock()
		if isCoordinator {
			go n.initiateGlobalCheckpoint()
		}
	}
}

// itemName is a nil-safe helper to get an item's name for logging.
func itemName(item *AuctionItem) string {
	if item == nil {
		return "<none>"
	}
	return item.Name
}
