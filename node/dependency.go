package node

func (n *Node) markDependency(address string) {
	if address == "" || address == n.Address {
		return
	}
	n.DepMutex.Lock()
	n.Dependencies[address] = true
	n.DepMutex.Unlock()
}

func (n *Node) dependencySnapshot() []string {
	n.DepMutex.Lock()
	defer n.DepMutex.Unlock()
	deps := make([]string, 0, len(n.Dependencies))
	for dep := range n.Dependencies {
		deps = append(deps, dep)
	}
	return deps
}

func (n *Node) clearDependenciesForParticipants(participants map[string]bool) {
	n.DepMutex.Lock()
	defer n.DepMutex.Unlock()
	for dep := range participants {
		delete(n.Dependencies, dep)
	}
}

func (n *Node) callPeer(address, method string, args interface{}, reply interface{}) error {
	err := n.Client.Call(address, method, args, reply)
	if err == nil {
		n.markDependency(address)
	}
	return err
}
