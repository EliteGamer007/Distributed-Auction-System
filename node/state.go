package node

import (
	"sync"
)

type AuctionState struct {
	mu         sync.Mutex
	HighestBid int
	Winner     string
	Active     bool
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
