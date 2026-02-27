package node

import (
	"sync"
)

// AuctionItem describes a single item being put up for auction.
type AuctionItem struct {
	ID           string
	Name         string
	Description  string
	Emoji        string
	StartingPrice int
	DurationSec  int
}

// ItemResult records the outcome of a completed auction item.
type ItemResult struct {
	Item       AuctionItem
	Winner     string
	WinningBid int
}

// ItemQueueState is the full shared state of the auction queue.
type ItemQueueState struct {
	mu                sync.Mutex
	Queue             []AuctionItem // remaining items (not yet started)
	CurrentItem       *AuctionItem  // nil when no active item
	CurrentHighestBid int
	CurrentWinner     string
	DeadlineUnix      int64 // Unix timestamp (seconds) when current item closes
	Active            bool  // false after all items are done
	Results           []ItemResult
}

type LamportClock struct {
	mu   sync.Mutex
	time int
}

func (c *LamportClock) Tick() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.time++
	return c.time
}

func (c *LamportClock) Update(receivedTime int) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if receivedTime > c.time {
		c.time = receivedTime
	}
	c.time++
	return c.time
}

func (c *LamportClock) Get() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.time
}
