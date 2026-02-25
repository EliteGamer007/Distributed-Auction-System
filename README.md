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
2. Run .\auction_node.exe --id Node1 --port 8001 --peers localhost:8002,localhost:8003,localhost:8004
3. Run .\auction_node.exe --id Node2 --port 8002 --peers localhost:8001,localhost:8003,localhost:8004
4. Run .\auction_node.exe --id Node3 --port 8003 --peers localhost:8001,localhost:8002,localhost:8004
5. Run .\auction_node.exe --id Node4 --port 8004 --peers localhost:8001,localhost:8002,localhost:8003

Open your browser and navigate to the UI for any of the nodes:
Node 1: http://localhost:8001
Node 2: http://localhost:8002
Node 3: http://localhost:8003
Node 4: http://localhost:8004