# Distributed Auction System

A fault-tolerant, real-time distributed online auction system built in **Go**. Four independent nodes communicate via **RPC** in an asynchronous message-passing environment, implementing textbook distributed systems algorithms end-to-end.

> Designed as a practical demonstration of distributed computing concepts: leader election, mutual exclusion, consensus, coordinated checkpointing, transaction logging, and fault recovery — all running on ordinary laptops connected over Wi-Fi.

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Algorithms Implemented](#algorithms-implemented)
3. [Project Structure](#project-structure)
4. [Prerequisites](#prerequisites)
5. [Quick Start (Single Machine)](#quick-start-single-machine)
6. [Manual Startup (4 Terminals)](#manual-startup-4-terminals)
7. [Running on 4 Laptops (LAN)](#running-on-4-laptops-lan)
8. [Web UI](#web-ui)
9. [HTTP API Reference](#http-api-reference)
10. [How a Bid Works (End-to-End)](#how-a-bid-works-end-to-end)
11. [Checkpointing & Recovery](#checkpointing--recovery)
12. [Transaction Logging](#transaction-logging)
13. [Fault Tolerance Scenarios](#fault-tolerance-scenarios)
14. [Troubleshooting](#troubleshooting)

---

## Architecture Overview

```
┌──────────┐    RPC     ┌──────────┐
│  Node 1  │◄──────────►│  Node 2  │
│  :8001   │            │  :8002   │
└────┬─────┘            └────┬─────┘
     │      ╲          ╱     │
     │ RPC   ╲  RPC   ╱ RPC │
     │        ╲      ╱      │
┌────┴─────┐   ╲    ╱  ┌────┴─────┐
│  Node 3  │◄───────►  │  Node 4  │  ← Highest rank = Coordinator
│  :8003   │            │  :8004   │
└──────────┘            └──────────┘
```

- **4 nodes**, each an independent Go process with its own HTTP server and RPC endpoint
- **Shared-nothing** — no shared memory, no shared disk; all coordination via message passing
- **Crash-stop failure model** — nodes may crash at any time; they recover from checkpoints on restart
- **Majority quorum** — 3 out of 4 nodes must agree to commit a bid
- **Lamport logical clocks** — total ordering of all events across nodes
- **Coordinator-based** — the highest-ranked node (elected via Bully algorithm) drives 2PC, checkpointing, and item queue progression

---

## Algorithms Implemented

| Algorithm | Purpose | File(s) |
|---|---|---|
| **Bully Election** | Elect coordinator (leader); re-elect on failure | `node/bully.go` |
| **Ricart–Agrawala** | Distributed mutual exclusion for serializing bid commits | `node/ricart_agrawala.go` |
| **Two-Phase Commit (2PC)** | Atomic bid consensus with majority quorum voting | `node/bid.go`, `node/rpc.go` |
| **Koo–Toueg Checkpointing** | Coordinated global checkpoint with dependency tracking | `node/checkpoint.go`, `node/dependency.go` |
| **Lamport Logical Clocks** | Causal event ordering across nodes | `node/state.go` |
| **Termination Detection** | ACK-based verification that all participants applied a decision | `node/bid.go` |
| **Transaction Logging** | Durable JSONL audit trail for every 2PC lifecycle event | `node/txnlog.go` |

---

## Project Structure

```
Distributed-Auction-System/
├── main.go                  # Entry point: CLI flag parsing, node construction
├── go.mod                   # Go module (auction_node)
├── start_nodes.ps1          # PowerShell one-click launcher (4 nodes)
├── .gitignore
├── README.md                # This file
├── Implementation details_utf8.txt  # Syllabus mapping document
├── node/
│   ├── state.go             # Core types: AuctionItem, ItemResult, LamportClock
│   ├── node.go              # Node struct, constructor, HTTP server, Start()
│   ├── bully.go             # Bully leader election + heartbeat protocol
│   ├── ricart_agrawala.go   # Ricart–Agrawala mutual exclusion
│   ├── bid.go               # 2PC bid proposal, ACK collection, retry logic
│   ├── rpc.go               # All RPC message types + handler methods
│   ├── client.go            # RPCClient: thin wrapper over Go net/rpc
│   ├── dependency.go        # callPeer() wrapper + dependency tracking for Koo–Toueg
│   ├── checkpoint.go        # Koo–Toueg coordinated checkpointing engine
│   ├── txnlog.go            # Durable JSONL transaction audit log
│   ├── queue.go             # Item queue, timer, anti-snipe, state sync
│   └── handlers.go          # HTTP handlers: /bid, /state, /admin/*, /checkpoint
├── checkpoints/             # (gitignored) JSON checkpoint files per node
└── txlogs/                  # (gitignored) JSONL transaction logs per node
```

---

## Prerequisites

- **Go 1.21+** installed and on PATH (`go version` to verify)
- **Windows** (startup script is PowerShell); the Go code itself is cross-platform
- **Ports 8001–8004** available on the machine (or on each laptop for LAN mode)

---

## Quick Start (Single Machine)

The easiest way to launch all 4 nodes:

```powershell
cd Distributed-Auction-System
powershell -ExecutionPolicy Bypass -File .\start_nodes.ps1
```

**What this does automatically:**
1. Stops any old `auction_node.exe` processes from a previous run
2. Preserves previous logs as `node1.last.log` … `node4.last.log`
3. Rebuilds a fresh `auction_node.exe` from source
4. Starts all 4 nodes on ports 8001–8004 with correct peer lists

**After startup, open any node's UI in your browser:**
- http://localhost:8001 (Node1)
- http://localhost:8002 (Node2)
- http://localhost:8003 (Node3)
- http://localhost:8004 (Node4)

Node4 (highest rank) will automatically win the Bully election and become the coordinator.

> **Tip:** This script avoids the common Windows error `unable to unlink old 'auction_node.exe'` by killing old processes before rebuilding.

---

## Manual Startup (4 Terminals)

If you prefer manual control, build first and then open **4 separate terminal windows**:

**Build:**
```powershell
go build -o auction_node.exe .
```

**Run each node:**
```powershell
# Terminal 1
.\auction_node.exe --id Node1 --host 0.0.0.0 --port 8001 --peers localhost:8002,localhost:8003,localhost:8004

# Terminal 2
.\auction_node.exe --id Node2 --host 0.0.0.0 --port 8002 --peers localhost:8001,localhost:8003,localhost:8004

# Terminal 3
.\auction_node.exe --id Node3 --host 0.0.0.0 --port 8003 --peers localhost:8001,localhost:8002,localhost:8004

# Terminal 4
.\auction_node.exe --id Node4 --host 0.0.0.0 --port 8004 --peers localhost:8001,localhost:8002,localhost:8003
```

### CLI Flags

| Flag | Description | Example |
|---|---|---|
| `--id` | Node identifier (must be `Node<N>`) | `Node1`, `Node4` |
| `--host` | Bind address | `0.0.0.0` (all interfaces) |
| `--port` | TCP port for HTTP + RPC | `8001` |
| `--peers` | Comma-separated peer addresses (exclude self) | `localhost:8002,localhost:8003,localhost:8004` |

Node rank is derived automatically from the ID suffix: `Node4` → rank 4 (highest wins election).

---

## Running on 4 Laptops (LAN)

To run the system across **4 physical laptops** on the same Wi-Fi network:

### Step 1: Find each laptop's IPv4 address
```powershell
ipconfig | Select-String "IPv4"
```

### Step 2: Allow ports through Windows Firewall
Run as **Administrator** on each laptop:
```powershell
New-NetFirewallRule -DisplayName "Auction Node" -Direction Inbound -Protocol TCP -LocalPort 8001-8004 -Action Allow
```

### Step 3: Build on each laptop
Copy the project folder to each laptop and run:
```powershell
cd Distributed-Auction-System
go build -o auction_node.exe .
```

### Step 4: Start each node with LAN IPs

Replace the IPs below with your actual addresses:

| Laptop | Command |
|---|---|
| A (Node1) | `.\auction_node.exe --id Node1 --host 0.0.0.0 --port 8001 --peers 192.168.1.12:8002,192.168.1.13:8003,192.168.1.14:8004` |
| B (Node2) | `.\auction_node.exe --id Node2 --host 0.0.0.0 --port 8002 --peers 192.168.1.11:8001,192.168.1.13:8003,192.168.1.14:8004` |
| C (Node3) | `.\auction_node.exe --id Node3 --host 0.0.0.0 --port 8003 --peers 192.168.1.11:8001,192.168.1.12:8002,192.168.1.14:8004` |
| D (Node4) | `.\auction_node.exe --id Node4 --host 0.0.0.0 --port 8004 --peers 192.168.1.11:8001,192.168.1.12:8002,192.168.1.13:8003` |

> **Important:** Each node's `--peers` list must contain all OTHER nodes, never itself.

### Step 5: Access the UI
Open `http://<any-laptop-ip>:<port>` from any device on the same network.

---

## Web UI

Every node serves a built-in web UI at its root URL (`http://localhost:800X/`):

- **Current item** being auctioned (name, description, starting price)
- **Highest bid** and current winner — updated in real-time
- **Countdown timer** showing time remaining
- **Bid form** — enter an amount and bidder name to place a bid
- **Past results** — completed items with winners and winning bids
- **Upcoming items** — items still in the queue

Any node can accept bids. Followers automatically forward bids to the coordinator via RPC.

---

## HTTP API Reference

All endpoints are available on every node (port 8001–8004).

### Place a Bid
```
POST /bid
Content-Type: application/x-www-form-urlencoded

amount=600&bidder=Alice
```
**Response (200):** `Bid committed by quorum and globally terminated`
**Error (400):** `Bid must be higher than current highest bid (or auction inactive)`

### Get Auction State
```
GET /state
```
Returns JSON with current item, highest bid, winner, deadline, queue length, results, and whether this node is the coordinator.

### Add an Item to the Queue
```
POST /admin/item
Content-Type: application/json

{"name": "Diamond Ring", "description": "2ct solitaire", "startingPrice": 5000, "durationSec": 120}
```

### Restart the Auction (Reset All Items)
```
POST /admin/auction
Content-Type: application/x-www-form-urlencoded

action=restart
```

### Start the Auction
```
POST /admin/auction
Content-Type: application/x-www-form-urlencoded

action=start
```

### View a Node's Checkpoint
```
GET /checkpoint
```
Returns the node's latest checkpoint as JSON (Lamport time, auction state, pending transactions).

---

## How a Bid Works (End-to-End)

This is the complete flow when a user submits a bid:

```
User ──POST /bid──► Node 2 (follower)
                        │
                        │ RPC: SubmitBidToCoordinator
                        ▼
                    Node 4 (coordinator)
                        │
            ┌───────────┼──── Ricart–Agrawala ────┐
            │     Acquire mutual exclusion         │
            │     (prevents concurrent 2PC)        │
            └───────────┼─────────────────────────┘
                        │
        ┌───── 2PC Phase 1: PREPARE (vote) ───────┐
        │               │                          │
        ▼               ▼                          ▼
    Node 1          Node 2              Node 3
    Vote: YES       Vote: YES           Vote: YES
        │               │                   │
        └───────────────┼───────────────────┘
                        │
              votes=4 ≥ quorum=3 → COMMIT
                        │
        ┌───── 2PC Phase 2: DECIDE (commit) ──────┐
        │               │                          │
        ▼               ▼                          ▼
    Node 1          Node 2              Node 3
    Apply bid       Apply bid           Apply bid
    Send ACK        Send ACK            Send ACK
        │               │                   │
        └───────────────┼───────────────────┘
                        │
              All ACKs received → TXN_TERMINATED
                        │
                   Return "Bid committed by quorum
                       and globally terminated"
```

**Key guarantees:**
- **Atomicity**: Either all quorum nodes apply the bid, or none do
- **Mutual exclusion**: Only one 2PC can run at a time (Ricart–Agrawala)
- **Termination detection**: Coordinator tracks ACKs from all participants; retries up to 5 times for missing ACKs
- **Anti-snipe**: If a bid lands with <15s remaining, the deadline extends by 15s

---

## Checkpointing & Recovery

### Koo–Toueg Coordinated Checkpointing

The system uses the **Koo–Toueg algorithm** for consistent global checkpoints:

1. **Dependency tracking**: Every RPC call from node A to node B records B as a "communication dependency" of A (via the `callPeer()` wrapper in `dependency.go`)

2. **Tentative phase**: The coordinator takes a tentative checkpoint and sends a `HandleKTTentativeCheckpoint` request to every node in its dependency set

3. **Recursive propagation**: Each receiving node tentatively checkpoints, then recursively sends the request to *its own* dependencies that haven't been visited yet — ensuring the checkpoint set includes all causally related nodes

4. **Finalize phase**: If all tentative checkpoints succeed, the coordinator broadcasts `COMMIT`; each node atomically promotes its tentative file to the stable checkpoint. On any failure, `ABORT` is broadcast and tentative files are deleted

5. **Serialization**: Only one checkpoint round runs at a time (enforced by `CkptMutex`)

### Checkpoint Contents

Each checkpoint (`checkpoints/checkpoint_NodeX.json`) stores:
- Node ID and Lamport timestamp
- Current auction item and highest bid
- Remaining item queue and completed results
- All pending (prepared but undecided) transactions
- Wall-clock timestamp

### Recovery on Restart

When a node starts, it:
1. Loads `checkpoints/checkpoint_NodeX.json` (if it exists)
2. Restores Lamport clock, auction state, and pending transactions
3. Rejoins the cluster and syncs with the coordinator via periodic state pulls (every 2 seconds)

### When Checkpoints Are Triggered

The coordinator triggers a Koo–Toueg global checkpoint:
- Every **30 seconds** (periodic timer)
- After every **item transition** (new item started, item finalized)
- After every **auction restart or item addition**

---

## Transaction Logging

Every 2PC lifecycle event is logged to a durable JSONL file at `txlogs/txn_NodeX.log`:

| Event | Meaning |
|---|---|
| `TXN_BEGIN` | Coordinator started a new 2PC round |
| `TXN_PREPARED` | Node stored the bid as a pending transaction |
| `TXN_PREPARE_VOTE_YES` | Participant voted YES in Phase 1 |
| `TXN_PREPARE_VOTE_NO` | Participant voted NO in Phase 1 |
| `TXN_COMMIT_APPLIED` | Node applied the committed bid to its state |
| `TXN_ABORT_APPLIED` | Node discarded the aborted bid |
| `TXN_DECIDE_ACK` | Coordinator received ACK from a participant |
| `TXN_DECIDE_ACK_SENT` | Participant applied the decision and sent ACK |
| `TXN_TERMINATED` | All participants have ACKed — transaction globally terminated |
| `TXN_TERMINATION_PENDING` | Some ACKs missing; retry loop started |
| `TXN_TERMINATION_RETRY` | Retry attempt for missing ACKs |
| `TXN_TERMINATION_INCOMPLETE` | Gave up after max retries; some participants unreachable |
| `TXN_STALE_ABORT` | Auto-aborted a prepared txn that never received a decision (timeout) |

Example log entry:
```json
{"timestampUnix":1741108800,"nodeId":"Node4","txnId":"Node4-42","event":"TXN_TERMINATED","message":"all participants ACKed (3/3)"}
```

---

## Fault Tolerance Scenarios

### Leader Crash
1. Followers detect missing heartbeats (3-second timeout)
2. Bully election starts — highest-rank surviving node wins
3. New coordinator resumes the item timer and checkpoint schedule
4. Followers auto-sync state from the new coordinator

### Participant Crash During Voting
- If a participant is unreachable during Phase 1, its vote counts as NO
- The coordinator still commits if it has a majority quorum (≥3 out of 4)
- Missing participants can retry receiving the decision via the ACK retry loop

### Participant Crash After Commit
- The coordinator retries `DecideBid` up to 5 times with 2-second intervals
- On recovery, the node restores from its checkpoint and syncs state from the coordinator
- Stale prepared transactions (>8 seconds without a decision) are auto-aborted

### Network Partition
- Nodes on the minority side lose heartbeats and trigger elections, but cannot form a quorum
- The majority partition continues operating normally
- On partition heal, the minority nodes receive coordinator announcements and resync

---

## Troubleshooting

### `error: unable to unlink old 'auction_node.exe'`
The executable is locked by a running process. Use the startup script (it kills old processes first), or manually stop them:
```powershell
Get-Process auction_node -ErrorAction SilentlyContinue | Stop-Process -Force
```

### Nodes can't connect on LAN
1. Verify all laptops are on the **same Wi-Fi network**
2. Check firewall rules: `Get-NetFirewallRule -DisplayName "Auction*"`
3. Test connectivity: `Test-NetConnection -ComputerName 192.168.1.12 -Port 8002`
4. Ensure `--peers` uses **IP addresses**, not `localhost`

### Bids rejected with "auction inactive"
The auction may have ended or the node's state isn't synced. Restart the auction:
```powershell
Invoke-WebRequest -Uri http://localhost:8004/admin/auction -Method Post -Body "action=restart" -ContentType "application/x-www-form-urlencoded"
```

### Checking logs for errors
```powershell
# View latest events on the coordinator (Node4)
Get-Content .\node4.log -Tail 30

# Search for errors across all node logs
Select-String -Path .\node*.log -Pattern "failed|error|timed out" -SimpleMatch

# View transaction log for Node4
Get-Content .\txlogs\txn_Node4.log -Tail 20

# View checkpoint for any node
Invoke-WebRequest http://localhost:8001/checkpoint -UseBasicParsing | Select-Object -ExpandProperty Content | ConvertFrom-Json
```

---

## License

MIT
