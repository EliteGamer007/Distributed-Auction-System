package node

// checkpoint.go — Koo-Toueg-style coordinated checkpointing.
//
// Round flow (initiator/coordinator driven):
//  1) Tentative phase: initiator takes tentative checkpoint and sends request
//     to nodes in its dependency set.
//  2) Propagation: each receiver tentatively checkpoints and recursively
//     requests all dependencies not already visited in the round.
//  3) Finalize phase: if all tentative requests ACK, initiator broadcasts
//     COMMIT (otherwise ABORT) to all round participants.
//  4) Commit moves tentative file atomically to stable checkpoint file.

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
	checkpointAckTimeout = 6 * time.Second
)

// CheckpointData is the full serialisable state of a node, written to disk.
type CheckpointData struct {
	NodeID            string                          `json:"nodeId"`
	LamportTime       int                             `json:"lamportTime"`
	CurrentItem       *AuctionItem                    `json:"currentItem"`
	RemainingQueue    []AuctionItem                   `json:"remainingQueue"`
	Results           []ItemResult                    `json:"results"`
	CurrentHighestBid int                             `json:"currentHighestBid"`
	CurrentWinner     string                          `json:"currentWinner"`
	DeadlineUnix      int64                           `json:"deadlineUnix"`
	Active            bool                            `json:"active"`
	PendingTxns       map[string]PendingTxnCheckpoint `json:"pendingTxns"`
	CheckpointTime    int64                           `json:"checkpointTime"` // wall-clock Unix
	LamportStamp      int                             `json:"lamportStamp"`   // Lamport time at checkpoint
}

type PendingTxnCheckpoint struct {
	Bid            BidArgs `json:"bid"`
	PreparedAtUnix int64   `json:"preparedAtUnix"`
}

// checkpointPath returns the file path for a node's checkpoint.
func checkpointPath(nodeID string) string {
	return filepath.Join(checkpointDir, fmt.Sprintf("checkpoint_%s.json", nodeID))
}

func tentativeCheckpointPath(nodeID, roundID string) string {
	return filepath.Join(checkpointDir, fmt.Sprintf("checkpoint_%s_%s.tentative.json", nodeID, roundID))
}

// saveCheckpoint writes data to checkpoints/<NodeID>.json atomically.
func saveCheckpointToPath(path string, data CheckpointData) error {
	if err := os.MkdirAll(checkpointDir, 0o755); err != nil {
		return fmt.Errorf("mkdir checkpoints: %w", err)
	}
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

func saveCheckpoint(data CheckpointData) error {
	return saveCheckpointToPath(checkpointPath(data.NodeID), data)
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

func (n *Node) buildCheckpointData() CheckpointData {
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
		PendingTxns:       map[string]PendingTxnCheckpoint{},
		CheckpointTime:    time.Now().Unix(),
	}
	if n.Queue.CurrentItem != nil {
		item := *n.Queue.CurrentItem
		data.CurrentItem = &item
	}
	n.Queue.mu.Unlock()

	n.TxnMutex.Lock()
	for txnID, pending := range n.PendingTxns {
		data.PendingTxns[txnID] = PendingTxnCheckpoint{
			Bid:            pending.Bid,
			PreparedAtUnix: pending.PreparedAt.Unix(),
		}
	}
	n.TxnMutex.Unlock()

	return data
}

// takeLocalCheckpoint snapshots this node's current state and saves it to disk.
func (n *Node) takeLocalCheckpoint() error {
	data := n.buildCheckpointData()

	if err := saveCheckpoint(data); err != nil {
		return err
	}
	log.Printf("[%s] 📸 Checkpoint saved (lamport=%d, item=%v, results=%d, pendingTxns=%d)\n",
		n.ID, data.LamportStamp, itemName(data.CurrentItem), len(data.Results), len(data.PendingTxns))
	return nil
}

func (n *Node) takeTentativeCheckpoint(roundID string) error {
	data := n.buildCheckpointData()
	if err := saveCheckpointToPath(tentativeCheckpointPath(n.ID, roundID), data); err != nil {
		return err
	}
	log.Printf("[%s] 📝 Tentative checkpoint taken (round=%s)\n", n.ID, roundID)
	return nil
}

func (n *Node) commitTentativeCheckpoint(roundID string) error {
	tentative := tentativeCheckpointPath(n.ID, roundID)
	finalPath := checkpointPath(n.ID)

	if _, err := os.Stat(tentative); os.IsNotExist(err) {
		return nil
	}
	tmp := finalPath + ".tmp"

	b, err := os.ReadFile(tentative)
	if err != nil {
		return fmt.Errorf("read tentative: %w", err)
	}
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return fmt.Errorf("write final tmp: %w", err)
	}
	if err := os.Rename(tmp, finalPath); err != nil {
		return fmt.Errorf("rename final checkpoint: %w", err)
	}
	_ = os.Remove(tentative)
	log.Printf("[%s] ✅ Committed checkpoint round=%s\n", n.ID, roundID)
	return nil
}

func (n *Node) abortTentativeCheckpoint(roundID string) {
	_ = os.Remove(tentativeCheckpointPath(n.ID, roundID))
	log.Printf("[%s] ❌ Aborted tentative checkpoint round=%s\n", n.ID, roundID)
}

func (n *Node) beginKTRound(roundID string) (*KTRoundState, bool) {
	n.KTMutex.Lock()
	defer n.KTMutex.Unlock()
	if existing, ok := n.KTRounds[roundID]; ok {
		return existing, false
	}
	state := &KTRoundState{Participants: map[string]bool{n.Address: true}}
	n.KTRounds[roundID] = state
	return state, true
}

func mapKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func sliceToSet(values []string) map[string]bool {
	set := map[string]bool{}
	for _, v := range values {
		set[v] = true
	}
	return set
}

func (n *Node) handleKTTentativeRequest(args KTTentativeArgs) (bool, []string, string) {
	n.Clock.Update(args.LamportTime)

	state, isNewRound := n.beginKTRound(args.RoundID)
	if isNewRound {
		if err := n.takeTentativeCheckpoint(args.RoundID); err != nil {
			return false, nil, err.Error()
		}
		state.TentativeTaken = true
	}

	visited := sliceToSet(args.Visited)
	visited[n.Address] = true

	deps := n.dependencySnapshot()
	participants := map[string]bool{n.Address: true}

	for _, dep := range deps {
		if visited[dep] {
			continue
		}
		nextVisited := mapKeys(visited)
		nextVisited = append(nextVisited, dep)

		var reply KTTentativeReply
		err := n.callPeer(dep, "NodeRPC.HandleKTTentativeCheckpoint", KTTentativeArgs{
			RoundID:     args.RoundID,
			Initiator:   args.Initiator,
			LamportTime: n.Clock.Get(),
			From:        n.Address,
			Visited:     nextVisited,
		}, &reply)
		if err != nil || !reply.OK {
			n.abortTentativeCheckpoint(args.RoundID)
			return false, nil, fmt.Sprintf("dependency %s failed tentative checkpoint", dep)
		}
		for _, p := range reply.Participants {
			participants[p] = true
		}
	}

	n.KTMutex.Lock()
	for p := range participants {
		state.Participants[p] = true
	}
	n.KTMutex.Unlock()

	return true, mapKeys(participants), ""
}

func (n *Node) finalizeKTRound(roundID string, commit bool) {
	n.KTMutex.Lock()
	state, ok := n.KTRounds[roundID]
	if !ok || state.Finalized {
		n.KTMutex.Unlock()
		return
	}
	participants := map[string]bool{}
	for p := range state.Participants {
		participants[p] = true
	}
	state.Finalized = true
	state.Committed = commit
	n.KTMutex.Unlock()

	if commit {
		if err := n.commitTentativeCheckpoint(roundID); err != nil {
			log.Printf("[%s] ⚠️ commit tentative failed (round=%s): %v\n", n.ID, roundID, err)
		}
		n.clearDependenciesForParticipants(participants)
	} else {
		n.abortTentativeCheckpoint(roundID)
	}

	n.KTMutex.Lock()
	delete(n.KTRounds, roundID)
	n.KTMutex.Unlock()
}

// initiateGlobalCheckpoint is called by the coordinator to checkpoint all nodes.
func (n *Node) initiateGlobalCheckpoint() {
	n.CkptMutex.Lock()
	if n.CkptInFlight {
		n.CkptMutex.Unlock()
		return
	}
	n.CkptInFlight = true
	n.CkptMutex.Unlock()
	defer func() {
		n.CkptMutex.Lock()
		n.CkptInFlight = false
		n.CkptMutex.Unlock()
	}()

	n.ElectionMutex.Lock()
	isCoordinator := n.Coordinator == n.ID
	n.ElectionMutex.Unlock()
	if !isCoordinator {
		return
	}

	lamport := n.Clock.Tick()
	roundID := fmt.Sprintf("%s-%d", n.ID, lamport)
	log.Printf("[%s] 🟢 Koo-Toueg checkpoint round start: %s\n", n.ID, roundID)

	ok, participants, reason := n.handleKTTentativeRequest(KTTentativeArgs{
		RoundID:     roundID,
		Initiator:   n.ID,
		LamportTime: lamport,
		From:        n.Address,
		Visited:     []string{n.Address},
	})

	participantSet := sliceToSet(participants)
	if !ok {
		log.Printf("[%s] ⚠️ Koo-Toueg tentative phase failed: %s\n", n.ID, reason)
		n.finalizeKTRound(roundID, false)
		return
	}

	n.finalizeKTRound(roundID, true)

	type finalizeResult struct {
		peer string
		err  error
	}
	finalizeCh := make(chan finalizeResult, len(participantSet))
	for peer := range participantSet {
		if peer == n.Address {
			continue
		}
		go func(p string) {
			var ack bool
			err := n.callPeer(p, "NodeRPC.HandleKTFinalizeCheckpoint", KTFinalizeArgs{RoundID: roundID, Commit: true}, &ack)
			if err != nil || !ack {
				finalizeCh <- finalizeResult{peer: p, err: fmt.Errorf("finalize failed")}
				return
			}
			finalizeCh <- finalizeResult{peer: p, err: nil}
		}(peer)
	}

	timer := time.NewTimer(checkpointAckTimeout)
	defer timer.Stop()
	remaining := len(participantSet) - 1
	for remaining > 0 {
		select {
		case res := <-finalizeCh:
			remaining--
			if res.err != nil {
				log.Printf("[%s] ⚠️ Koo-Toueg finalize NACK from %s\n", n.ID, res.peer)
			} else {
				log.Printf("[%s] ✅ Koo-Toueg finalize ACK from %s\n", n.ID, res.peer)
			}
		case <-timer.C:
			log.Printf("[%s] ⚠️ Koo-Toueg finalize timed out with %d pending\n", n.ID, remaining)
			remaining = 0
		}
	}

	log.Printf("[%s] 🏁 Koo-Toueg checkpoint round committed: %s participants=%d\n",
		n.ID, roundID, len(participantSet))
}

// runPeriodicCheckpointing triggers a global checkpoint every 30s (coordinator only).
func (n *Node) runPeriodicCheckpointing() {
	ticker := time.NewTicker(checkpointInterval)
	defer ticker.Stop()
	for range ticker.C {
		n.ElectionMutex.Lock()
		isCoordinator := n.Coordinator == n.ID
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
