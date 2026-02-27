package node

// rpc.go — All RPC message types and NodeRPC handler methods.

// ── Types ─────────────────────────────────────────────────────────────────────

type BidArgs struct {
	Amount int
	Bidder string
}

type PrepareArgs struct {
	TxnID     string
	Bid       BidArgs
	Timestamp int
}

type PrepareReply struct {
	Vote   bool
	Reason string
}

type DecisionArgs struct {
	TxnID  string
	Commit bool
	Bid    BidArgs
	Leader string
}

type CoordinatorBidReply struct {
	Accepted bool
	Message  string
}

type EmptyArgs struct{}

type QueueSnapshot struct {
	CurrentItem       *AuctionItem
	CurrentHighestBid int
	CurrentWinner     string
	DeadlineUnix      int64
	Active            bool
	QueueLen          int
	RemainingItems    []AuctionItem
	Results           []ItemResult
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// SubmitBidToCoordinator is called by a follower to forward a bid to the leader.
func (rp *NodeRPC) SubmitBidToCoordinator(args BidArgs, reply *CoordinatorBidReply) error {
	rp.node.ElectionMutex.Lock()
	isCoordinator := rp.node.Coordinator == "" || rp.node.Coordinator == rp.node.ID
	rp.node.ElectionMutex.Unlock()

	if !isCoordinator {
		reply.Accepted = false
		reply.Message = "This node is not the coordinator"
		return nil
	}
	accepted, message := rp.node.ProposeBid(args.Amount, args.Bidder)
	reply.Accepted = accepted
	reply.Message = message
	return nil
}

// PrepareBid is Phase-1 of 2PC: a peer votes yes/no on a proposed bid.
func (rp *NodeRPC) PrepareBid(args PrepareArgs, reply *PrepareReply) error {
	rp.node.Clock.Update(args.Timestamp)
	if !rp.node.canPrepareBid(args.Bid) {
		reply.Vote = false
		reply.Reason = "bid not higher, auction inactive, or time expired"
		return nil
	}
	rp.node.rememberPendingTxn(args.TxnID, args.Bid)
	reply.Vote = true
	reply.Reason = "prepared"
	return nil
}

// DecideBid is Phase-2 of 2PC: apply commit or abort.
func (rp *NodeRPC) DecideBid(args DecisionArgs, reply *bool) error {
	rp.node.applyDecision(args.TxnID, args.Commit, args.Bid)
	*reply = true
	return nil
}

// GetQueueState lets a follower pull a full state snapshot from the coordinator.
func (rp *NodeRPC) GetQueueState(_ EmptyArgs, reply *QueueSnapshot) error {
	*reply = rp.node.buildQueueSnapshot()
	return nil
}

// SyncQueueState lets the coordinator push a state snapshot to followers.
func (rp *NodeRPC) SyncQueueState(snap QueueSnapshot, reply *bool) error {
	rp.node.applyQueueSnapshot(snap)
	*reply = true
	return nil
}

// HandleRARequest handles a Ricart-Agrawala mutual exclusion request.
func (rp *NodeRPC) HandleRARequest(args RAMessage, reply *bool) error {
	*reply = rp.node.RA.ReceiveRequest(args)
	return nil
}

// HandleRADeferredReply sends a deferred RA reply after releasing the CS.
func (rp *NodeRPC) HandleRADeferredReply(args RAMessage, reply *bool) error {
	rp.node.RA.HandleRAReply()
	*reply = true
	return nil
}

// HandleBid is a legacy direct-propagation handler, kept for compatibility.
func (rp *NodeRPC) HandleBid(args BidArgs, reply *bool) error {
	rp.node.Queue.mu.Lock()
	if rp.node.Queue.Active && rp.node.Queue.CurrentItem != nil && args.Amount > rp.node.Queue.CurrentHighestBid {
		rp.node.Queue.CurrentHighestBid = args.Amount
		rp.node.Queue.CurrentWinner = args.Bidder
	}
	rp.node.Queue.mu.Unlock()
	*reply = true
	return nil
}
