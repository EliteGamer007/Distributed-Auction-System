package node

import (
	"log"
	"sync"
)

type RAMessage struct {
	Timestamp int
	NodeID    string
}

type RAManager struct {
	mu            sync.Mutex
	NodeID        string
	Peers         []string
	Clock         *LamportClock
	RequestTime   int
	RequestingCS  bool
	RepliesNeeded int
	DeferredReply []string
	Client        *RPCClient
	ReplyChan     chan struct{}
}

func NewRAManager(nodeID string, peers []string, clock *LamportClock, client *RPCClient) *RAManager {
	return &RAManager{
		NodeID:    nodeID,
		Peers:     peers,
		Clock:     clock,
		Client:    client,
		ReplyChan: make(chan struct{}, len(peers)),
	}
}

func (ra *RAManager) RequestCS() {
	ra.mu.Lock()
	ra.RequestingCS = true
	ra.RequestTime = ra.Clock.Tick()
	ra.RepliesNeeded = len(ra.Peers)
	ra.mu.Unlock()

	log.Printf("[%s] Requesting Critical Section at Time %d\n", ra.NodeID, ra.RequestTime)

	for _, peer := range ra.Peers {
		go func(p string) {
			req := RAMessage{Timestamp: ra.RequestTime, NodeID: ra.NodeID}
			var reply bool
			err := ra.Client.Call(p, "NodeRPC.HandleRARequest", req, &reply)
			if err != nil {
				log.Printf("[%s] Failed to contact %s: %v", ra.NodeID, p, err)
				ra.HandleRAReply() // Proceed even if node is down
			} else if reply {
				ra.HandleRAReply()
			}
		}(peer)
	}

	for i := 0; i < len(ra.Peers); i++ {
		<-ra.ReplyChan
	}
	log.Printf("[%s] Entered Critical Section\n", ra.NodeID)
}

func (ra *RAManager) HandleRAReply() {
	ra.mu.Lock()
	defer ra.mu.Unlock()
	ra.RepliesNeeded--
	if ra.RepliesNeeded >= 0 {
		ra.ReplyChan <- struct{}{}
	}
}

func (ra *RAManager) ReceiveRequest(req RAMessage) bool {
	ra.mu.Lock()
	defer ra.mu.Unlock()

	ra.Clock.Update(req.Timestamp)

	deferReply := ra.RequestingCS && ((ra.RequestTime < req.Timestamp) || (ra.RequestTime == req.Timestamp && ra.NodeID < req.NodeID))

	if deferReply {
		log.Printf("[%s] Deferring reply to %s\n", ra.NodeID, req.NodeID)
		ra.DeferredReply = append(ra.DeferredReply, req.NodeID)
		return false
	}
	log.Printf("[%s] Replying to %s immediately\n", ra.NodeID, req.NodeID)
	return true
}

func (ra *RAManager) ReleaseCS() {
	ra.mu.Lock()
	ra.RequestingCS = false
	deferred := ra.DeferredReply
	ra.DeferredReply = nil
	ra.mu.Unlock()

	log.Printf("[%s] Releasing Critical Section, replying to %d deferred requests\n", ra.NodeID, len(deferred))
	for _, peer := range deferred {
		go func(p string) {
			var reply bool
			ra.Client.Call(p, "NodeRPC.HandleRADeferredReply", RAMessage{NodeID: ra.NodeID}, &reply)
		}(peer)
	}
}
