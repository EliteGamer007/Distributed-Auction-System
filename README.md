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
________________________________________
🏗 2️⃣ System Model
•	Asynchronous distributed system
•	Message-passing communication (Go RPC)
•	Crash-stop failure model
•	No shared memory
•	Logical clock-based ordering
•	Majority quorum (3/4)
________________________________________
🧠 3️⃣ Concepts Covered (Mapped to Syllabus)
________________________________________
🔹 A. Models of Computation
✔ Shared-nothing architecture
✔ Message-passing system
✔ Asynchronous system behavior
✔ RPC-based communication
________________________________________
🔹 B. Logical Time & Event Ordering
Implemented using:
•	Lamport Logical Clocks
•	Timestamped messages
•	Total ordering of bids
Purpose:
•	Resolve simultaneous bids
•	Maintain consistent event ordering
________________________________________
🔹 C. Distributed Mutual Exclusion
Algorithm used:
•	Ricart–Agrawala
Used to:
•	Prevent simultaneous bid processing
•	Ensure one transaction enters commit phase at a time
Demonstrates:
•	Fairness
•	No centralized locking
•	Deadlock-free protocol
________________________________________
🔹 D. Leader Election
Algorithm used:
•	Bully Algorithm
Used to:
•	Elect coordinator node
•	Replace failed leader
•	Maintain availability
Demonstrates:
•	Failure detection
•	Re-election
•	Liveness guarantee
________________________________________
🔹 E. Consensus & Agreement Problem
Consensus style:
•	Majority quorum-based agreement
Implementation:
•	Mini Two-Phase Commit
•	Voting phase
•	Commit/Abort decision
Demonstrates:
•	Agreement problem
•	Quorum logic
•	Strong consistency model
________________________________________
🔹 F. Commit Protocol
Two-Phase Commit (2PC):
Phase 1: PREPARE (Voting)
Phase 2: COMMIT or ABORT
Demonstrates:
•	Atomicity
•	Consistency
•	Blocking problem of 2PC
________________________________________
🔹 G. Fault Tolerance
Implemented using:
✔ Crash-stop model
✔ Leader re-election
✔ Transaction logs
✔ Recovery from checkpoint
✔ Timeout detection
Failure Scenarios Handled:
•	Leader crash before commit
•	Participant crash during voting
•	Network delay simulation
________________________________________
🔹 H. Coordinated Checkpointing
Mechanism:
•	Leader initiates global checkpoint
•	All nodes save:
o	Highest bid
o	Bidder
o	Logical clock
o	Pending transactions
Checkpoint ensures:
•	Consistent recovery state
•	No orphan processes
•	No inconsistent rollbacks
Demonstrates:
•	Coordinated checkpoint protocol
•	Recovery model
•	Consistent global state
________________________________________
🔹 I. Termination Detection
After commit:
•	Leader waits for ACK from all participants
•	Once all ACK received → transaction considered globally terminated
Demonstrates:
•	Distributed termination detection
•	Completion guarantees
________________________________________
🔹 J. Replica Management & Consistency
•	All nodes maintain replicated auction state
•	Commit only on majority agreement
•	Strong consistency model
Consistency Type:
•	Linearizable updates via quorum commit
________________________________________
🔹 K. Concurrency Control
•	Mutual exclusion ensures serializability
•	Logical clock ordering ensures deterministic processing
Equivalent to:
•	Strict serializable execution
________________________________________
🔹 L. Fault Recovery
Upon restart:
1.	Load checkpoint
2.	Rejoin cluster
3.	Sync with leader
4.	Resume normal operation
Demonstrates:
•	Recovery protocol
•	State reconciliation
________________________________________
🔹 M. Cloud & Distributed System Concepts
•	Cluster-style architecture
•	Coordinator-based transaction manager
•	Similar to distributed DB transaction layer
Conceptually inspired by:
•	Google Spanner
•	Amazon Aurora

