## Quick Start (Recommended)

Use the single starter script to cleanly relaunch the full 4-node cluster:

```powershell
powershell -ExecutionPolicy Bypass -File .\start_nodes.ps1
```

What this does automatically:
- Stops old `auction_node.exe` / `distributed-auction.exe` processes
- Preserves previous logs as `node1.last.log` ... `node4.last.log`
- Deletes stale executables and rebuilds a fresh `auction_node.exe`
- Starts all 4 nodes on ports `8001`-`8004`

This avoids the common Windows lock issue during checkout/rebuild:
`error: unable to unlink old 'auction_node.exe'`

Open any node UI after startup:
- http://localhost:8001
- http://localhost:8002
- http://localhost:8003
- http://localhost:8004

Problem Statement
Design and implement a distributed online auction system running on 4 independent nodes (laptops) communicating via RPC in an asynchronous message-passing environment.
The system must:
•	Ensure consistent agreement on highest bid
•	Prevent concurrent conflicting updates
•	Tolerate leader and participant failures
•	Recover using coordinated checkpointing
•	Detect transaction termination
•	Use majority-based consensus for committing bids
•	Maintain replica consistency across all nodes
The system simulates a simplified distributed transaction manager inspired by:
•	Google Spanner
•	Amazon Aurora

### ⭐ New Features Added
- **Leader Election via Bully Algorithm**: Ranks are parsed automatically from node IDs (e.g., Node4 > Node1). The cluster dynamically elects the highest-ranking node as the coordinator.
- **Failure Detection**: The Leader sends periodic heartbeats. Followers detect leader failure via timeouts and automatically hold a re-election.

HOW TO RUN FOR DUMMIES(Not Vishnu):
1. Open 4 cmd prompts
2. Run .\auction_node.exe --id Node1 --host 0.0.0.0 --port 8001 --peers localhost:8002,localhost:8003,localhost:8004
3. Run .\auction_node.exe --id Node2 --host 0.0.0.0 --port 8002 --peers localhost:8001,localhost:8003,localhost:8004
4. Run .\auction_node.exe --id Node3 --host 0.0.0.0 --port 8003 --peers localhost:8001,localhost:8002,localhost:8004
5. Run .\auction_node.exe --id Node4 --host 0.0.0.0 --port 8004 --peers localhost:8001,localhost:8002,localhost:8003

Open your browser and navigate to the UI for any of the nodes:
Node 1: http://localhost:8001
Node 2: http://localhost:8002
Node 3: http://localhost:8003
Node 4: http://localhost:8004

### Running across 4 laptops on same Wi-Fi (LAN)
Use each laptop's local IPv4 address in `--peers`, and keep ports mapped by node ID:
- Node1 -> `:8001`
- Node2 -> `:8002`
- Node3 -> `:8003`
- Node4 -> `:8004`

Example (replace with your actual IPs):
- Laptop A (Node1):
	`.\auction_node.exe --id Node1 --host 0.0.0.0 --port 8001 --peers 192.168.1.12:8002,192.168.1.13:8003,192.168.1.14:8004`
- Laptop B (Node2):
	`.\auction_node.exe --id Node2 --host 0.0.0.0 --port 8002 --peers 192.168.1.11:8001,192.168.1.13:8003,192.168.1.14:8004`
- Laptop C (Node3):
	`.\auction_node.exe --id Node3 --host 0.0.0.0 --port 8003 --peers 192.168.1.11:8001,192.168.1.12:8002,192.168.1.14:8004`
- Laptop D (Node4):
	`.\auction_node.exe --id Node4 --host 0.0.0.0 --port 8004 --peers 192.168.1.11:8001,192.168.1.12:8002,192.168.1.13:8003`

Make sure Windows Firewall allows inbound TCP on ports `8001-8004`.