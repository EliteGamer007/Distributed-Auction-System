package node

import (
	"log"
	"time"
)

type BullyMessage struct {
	NodeID string
	Rank   int
}

func (n *Node) StartElection() {
	log.Printf("[%s] Starting election (Rank: %d)\n", n.ID, n.Rank)

	receivedOK := false
	for _, peerAddress := range n.Peers {
		go func(addr string) {
			var ok bool
			err := n.Client.Call(addr, "NodeRPC.HandleElection", BullyMessage{NodeID: n.ID, Rank: n.Rank}, &ok)
			if err == nil && ok {
				n.ElectionMutex.Lock()
				receivedOK = true
				n.ElectionMutex.Unlock()
			}
		}(peerAddress)
	}

	// Wait for OK responses
	time.Sleep(2 * time.Second)

	n.ElectionMutex.Lock()
	isHighest := !receivedOK
	n.ElectionMutex.Unlock()

	if isHighest {
		log.Printf("[%s] No higher nodes, becoming leader!\n", n.ID)

		n.ElectionMutex.Lock()
		n.Coordinator = n.ID
		n.ElectionMutex.Unlock()

		// Broadcast coordinator
		for _, peerAddress := range n.Peers {
			go func(addr string) {
				var dummy bool
				n.Client.Call(addr, "NodeRPC.HandleCoordinator", BullyMessage{NodeID: n.ID, Rank: n.Rank}, &dummy)
			}(peerAddress)
		}

		// Leader broadcasts heartbeats continuously
		go n.BroadcastHeartbeats()

		// Start / resume the item queue timer
		go n.OnBecomeCoordinator()
	}
}

func (n *Node) BroadcastHeartbeats() {
	for {
		n.ElectionMutex.Lock()
		if n.Coordinator != n.ID {
			n.ElectionMutex.Unlock()
			break // stop sending heartbeats if no longer leader
		}
		n.ElectionMutex.Unlock()

		for _, peerAddress := range n.Peers {
			go func(addr string) {
				var dummy bool
				n.Client.Call(addr, "NodeRPC.HandleHeartbeat", BullyMessage{NodeID: n.ID, Rank: n.Rank}, &dummy)
			}(peerAddress)
		}

		time.Sleep(1 * time.Second)
	}
}

func (n *Node) MonitorLeader() {
	// Trigger an initial election on startup
	n.StartElection()

	for {
		n.ElectionMutex.Lock()
		isLeader := (n.Coordinator == n.ID)
		n.ElectionMutex.Unlock()

		if isLeader {
			time.Sleep(2 * time.Second)
			continue
		}

		select {
		case <-n.LeaderChan:
			// Heartbeat received, reset timeout
		case <-time.After(3 * time.Second):
			// Timeout triggered!
			log.Printf("[%s] Failure detected: leader heartbeat timed out\n", n.ID)
			n.StartElection()
		}
	}
}

// RPC Handlers

func (rp *NodeRPC) HandleElection(args BullyMessage, reply *bool) error {
	rp.node.ElectionMutex.Lock()
	defer rp.node.ElectionMutex.Unlock()

	if rp.node.Rank > args.Rank {
		*reply = true // Meaning "I will take over"
		// Only start election if we haven't already. To simplify, we can just start it. The timer in StartElection will serialize things.
		go rp.node.StartElection()
	} else {
		*reply = false
	}
	return nil
}

func (rp *NodeRPC) HandleCoordinator(args BullyMessage, reply *bool) error {
	rp.node.ElectionMutex.Lock()
	defer rp.node.ElectionMutex.Unlock()

	if rp.node.Coordinator != args.NodeID {
		rp.node.Coordinator = args.NodeID
		log.Printf("[%s] New leader elected: %s\n", rp.node.ID, args.NodeID)

		// Flush LeaderChan to avoid stale heartbeats, but a non-blocking read is fine
		select {
		case <-rp.node.LeaderChan:
		default:
		}
	}
	*reply = true
	return nil
}

func (rp *NodeRPC) HandleHeartbeat(args BullyMessage, reply *bool) error {
	// Discard heartbeat if it's from a lower rank node proposing themselves as leader mistakenly
	if args.Rank < rp.node.Rank && rp.node.Coordinator == rp.node.ID {
		*reply = false
		return nil
	}

	select {
	case rp.node.LeaderChan <- true:
	default:
	}

	*reply = true
	return nil
}
