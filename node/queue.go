package node

// queue.go — Item queue management: seeding, coordinator timer, finalization,
// snapshot building, and follower sync.

import (
	"fmt"
	"log"
	"time"
)

// defaultItems returns the pre-seeded list of auction items.
func defaultItems() []AuctionItem {
	return []AuctionItem{
		{ID: "item-1", Name: "Vintage Rolex Watch", Description: "1962 Submariner, excellent condition", Emoji: "⌚", StartingPrice: 500, DurationSec: 120},
		{ID: "item-2", Name: "Oil Painting", Description: "Original 18th-century landscape on canvas", Emoji: "🖼️", StartingPrice: 300, DurationSec: 120},
		{ID: "item-3", Name: "Limited Sneakers", Description: "Nike Air Jordan 1 OG, DS size 10", Emoji: "👟", StartingPrice: 200, DurationSec: 120},
		{ID: "item-4", Name: "Gaming Laptop", Description: "ASUS ROG, RTX 4090, 32GB RAM", Emoji: "💻", StartingPrice: 1000, DurationSec: 120},
		{ID: "item-5", Name: "Fender Guitar", Description: "1965 Fender Stratocaster, sunburst finish", Emoji: "🎸", StartingPrice: 800, DurationSec: 120},
		{ID: "item-6", Name: "Rare Gold Coin", Description: "1920 St. Gaudens Double Eagle, MS65", Emoji: "🪙", StartingPrice: 1500, DurationSec: 120},
	}
}

const antiSnipeWindow = int64(15) // seconds — reset timer if bid placed this close to deadline

// maybeExtendDeadline resets the current item's deadline to antiSnipeWindow seconds
// from now if a bid was placed within the anti-snipe window. Called by coordinator only.
func (n *Node) maybeExtendDeadline() {
	n.Queue.mu.Lock()
	if n.Queue.CurrentItem == nil || !n.Queue.Active {
		n.Queue.mu.Unlock()
		return
	}
	remaining := n.Queue.DeadlineUnix - time.Now().Unix()
	if remaining >= antiSnipeWindow {
		n.Queue.mu.Unlock()
		return
	}
	n.Queue.DeadlineUnix = time.Now().Unix() + antiSnipeWindow
	log.Printf("[%s] ⏱  Anti-snipe: extended deadline by %ds (was %ds left)\n",
		n.ID, antiSnipeWindow, remaining)
	n.Queue.mu.Unlock()
	n.broadcastQueueState()
}

// startNextItem is called only by the coordinator to advance the queue.
func (n *Node) startNextItem() {
	n.Queue.mu.Lock()

	if len(n.Queue.Queue) == 0 {
		n.Queue.CurrentItem = nil
		n.Queue.Active = false
		n.Queue.DeadlineUnix = 0
		n.Queue.mu.Unlock()
		log.Printf("[%s] All auction items completed\n", n.ID)
		n.broadcastQueueState()
		return
	}

	next := n.Queue.Queue[0]
	n.Queue.Queue = n.Queue.Queue[1:]
	n.Queue.CurrentItem = &next
	n.Queue.CurrentHighestBid = next.StartingPrice - 1
	n.Queue.CurrentWinner = ""
	n.Queue.DeadlineUnix = time.Now().Unix() + int64(next.DurationSec)
	n.Queue.mu.Unlock()

	log.Printf("[%s] Started auction for: %s (deadline in %ds)\n", n.ID, next.Name, next.DurationSec)
	n.broadcastQueueState()
	go n.initiateGlobalCheckpoint()
	go n.runItemTimer(next.ID, n.Queue.DeadlineUnix)
}

// runItemTimer sleeps until the deadline, then finalizes the item and advances the queue.
func (n *Node) runItemTimer(itemID string, deadlineUnix int64) {
	if dur := time.Until(time.Unix(deadlineUnix, 0)); dur > 0 {
		time.Sleep(dur)
	}

	n.ElectionMutex.Lock()
	isCoordinator := n.Coordinator == "" || n.Coordinator == n.ID
	n.ElectionMutex.Unlock()
	if !isCoordinator {
		return
	}

	n.Queue.mu.Lock()
	if n.Queue.CurrentItem == nil || n.Queue.CurrentItem.ID != itemID || n.Queue.DeadlineUnix != deadlineUnix {
		n.Queue.mu.Unlock()
		return
	}
	n.finalizeCurrentItemLocked()
	n.Queue.mu.Unlock()

	n.startNextItem()
}

// finalizeCurrentItemLocked records the result of the current item. Must hold Queue.mu.
func (n *Node) finalizeCurrentItemLocked() {
	if n.Queue.CurrentItem == nil {
		return
	}
	result := ItemResult{
		Item:       *n.Queue.CurrentItem,
		Winner:     n.Queue.CurrentWinner,
		WinningBid: n.Queue.CurrentHighestBid,
	}
	if result.WinningBid <= result.Item.StartingPrice-1 {
		result.Winner = "No bids"
		result.WinningBid = 0
	}
	n.Queue.Results = append(n.Queue.Results, result)
	log.Printf("[%s] Finalized: %s → winner=%s bid=%d\n", n.ID, result.Item.Name, result.Winner, result.WinningBid)
	n.Queue.CurrentItem = nil
	// Checkpoint after every item closes so we never lose a result.
	go n.initiateGlobalCheckpoint()
}

// broadcastQueueState pushes a snapshot to all peer nodes.
func (n *Node) broadcastQueueState() {
	snap := n.buildQueueSnapshot()
	for _, peer := range n.Peers {
		go func(p string) {
			var ok bool
			_ = n.Client.Call(p, "NodeRPC.SyncQueueState", snap, &ok)
		}(peer)
	}
}

// buildQueueSnapshot returns a serialisable copy of the current queue state.
func (n *Node) buildQueueSnapshot() QueueSnapshot {
	n.Queue.mu.Lock()
	defer n.Queue.mu.Unlock()

	snap := QueueSnapshot{
		CurrentHighestBid: n.Queue.CurrentHighestBid,
		CurrentWinner:     n.Queue.CurrentWinner,
		DeadlineUnix:      n.Queue.DeadlineUnix,
		Active:            n.Queue.Active,
		QueueLen:          len(n.Queue.Queue),
		Results:           append([]ItemResult(nil), n.Queue.Results...),
		RemainingItems:    append([]AuctionItem(nil), n.Queue.Queue...),
	}
	if n.Queue.CurrentItem != nil {
		item := *n.Queue.CurrentItem
		snap.CurrentItem = &item
	}
	return snap
}

// applyQueueSnapshot overwrites local state with the coordinator's snapshot.
func (n *Node) applyQueueSnapshot(snap QueueSnapshot) {
	n.Queue.mu.Lock()
	defer n.Queue.mu.Unlock()
	n.Queue.CurrentItem = snap.CurrentItem
	n.Queue.CurrentHighestBid = snap.CurrentHighestBid
	n.Queue.CurrentWinner = snap.CurrentWinner
	n.Queue.DeadlineUnix = snap.DeadlineUnix
	n.Queue.Active = snap.Active
	n.Queue.Queue = snap.RemainingItems
	if len(snap.Results) > len(n.Queue.Results) {
		n.Queue.Results = snap.Results
	}
}

// periodicStateSync pulls state from the coordinator every 2 seconds (follower only).
func (n *Node) periodicStateSync() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		coordinatorAddress, isLocalCoordinator := n.getCoordinatorAddress()
		if isLocalCoordinator || coordinatorAddress == "" {
			continue
		}
		var snap QueueSnapshot
		if err := n.Client.Call(coordinatorAddress, "NodeRPC.GetQueueState", EmptyArgs{}, &snap); err != nil {
			continue
		}
		n.applyQueueSnapshot(snap)
	}
}

// OnBecomeCoordinator is called after a Bully election win to (re)start the item timer.
func (n *Node) OnBecomeCoordinator() {
	n.Queue.mu.Lock()
	hasItem := n.Queue.CurrentItem != nil
	deadlineSet := n.Queue.DeadlineUnix > 0
	n.Queue.mu.Unlock()

	switch {
	case hasItem && deadlineSet:
		// Resume existing timer
		n.Queue.mu.Lock()
		itemID := n.Queue.CurrentItem.ID
		deadline := n.Queue.DeadlineUnix
		n.Queue.mu.Unlock()
		go n.runItemTimer(itemID, deadline)

	case hasItem:
		// No deadline yet — set one now
		n.Queue.mu.Lock()
		dur := n.Queue.CurrentItem.DurationSec
		n.Queue.DeadlineUnix = time.Now().Unix() + int64(dur)
		itemID := n.Queue.CurrentItem.ID
		deadline := n.Queue.DeadlineUnix
		n.Queue.mu.Unlock()
		n.broadcastQueueState()
		go n.runItemTimer(itemID, deadline)

	default:
		n.startNextItem()
	}
}

func (n *Node) addItemAndBroadcast(name, description string, startingPrice, durationSec int) (bool, string) {
	if name == "" || description == "" || startingPrice <= 0 || durationSec <= 0 {
		return false, "name, description, starting price, and duration are required"
	}

	n.RA.RequestCS()
	defer n.RA.ReleaseCS()

	n.Queue.mu.Lock()
	newID := fmt.Sprintf("item-%d", len(n.Queue.Queue)+len(n.Queue.Results)+2)
	if n.Queue.CurrentItem == nil {
		newID = "item-1"
	}
	item := AuctionItem{
		ID:            newID,
		Name:          name,
		Description:   description,
		Emoji:         "",
		StartingPrice: startingPrice,
		DurationSec:   durationSec,
	}
	n.Queue.Queue = append(n.Queue.Queue, item)
	n.Queue.mu.Unlock()

	n.broadcastQueueState()
	go n.initiateGlobalCheckpoint()
	return true, "Item added to queue"
}

func (n *Node) startAuctionAndBroadcast() (bool, string) {
	n.RA.RequestCS()
	defer n.RA.ReleaseCS()

	n.Queue.mu.Lock()
	if n.Queue.Active && n.Queue.CurrentItem != nil && n.Queue.DeadlineUnix > time.Now().Unix() {
		n.Queue.mu.Unlock()
		return true, "Auction already running"
	}

	if n.Queue.CurrentItem == nil {
		if len(n.Queue.Queue) == 0 {
			n.Queue.Active = false
			n.Queue.mu.Unlock()
			return false, "No items available to start"
		}
		next := n.Queue.Queue[0]
		n.Queue.Queue = n.Queue.Queue[1:]
		n.Queue.CurrentItem = &next
		n.Queue.CurrentHighestBid = next.StartingPrice - 1
		n.Queue.CurrentWinner = ""
	}

	n.Queue.Active = true
	dur := n.Queue.CurrentItem.DurationSec
	n.Queue.DeadlineUnix = time.Now().Unix() + int64(dur)
	itemID := n.Queue.CurrentItem.ID
	deadline := n.Queue.DeadlineUnix
	n.Queue.mu.Unlock()

	n.broadcastQueueState()
	go n.initiateGlobalCheckpoint()
	go n.runItemTimer(itemID, deadline)
	return true, "Auction started"
}

func (n *Node) restartAuctionAndBroadcast() (bool, string) {
	n.RA.RequestCS()
	defer n.RA.ReleaseCS()

	items := defaultItems()
	first := items[0]

	n.Queue.mu.Lock()
	n.Queue.Queue = items[1:]
	n.Queue.CurrentItem = &first
	n.Queue.CurrentHighestBid = first.StartingPrice - 1
	n.Queue.CurrentWinner = ""
	n.Queue.Results = nil
	n.Queue.Active = true
	n.Queue.DeadlineUnix = time.Now().Unix() + int64(first.DurationSec)
	itemID := first.ID
	deadline := n.Queue.DeadlineUnix
	n.Queue.mu.Unlock()

	n.broadcastQueueState()
	go n.initiateGlobalCheckpoint()
	go n.runItemTimer(itemID, deadline)
	return true, "Auction restarted"
}
