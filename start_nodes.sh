#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

PEERS="localhost:8001,localhost:8002,localhost:8003,localhost:8004"
EXE="$SCRIPT_DIR/auction_node"
KEEP_CHECKPOINTS=false

# Parse flags
for arg in "$@"; do
    case "$arg" in
        --keep-checkpoints) KEEP_CHECKPOINTS=true ;;
        *) echo "Unknown argument: $arg"; exit 1 ;;
    esac
done

# ── [1/5] Stop any running node processes ─────────────────────────────────────
echo "[1/5] Stopping old node processes..."
pkill -f "auction_node --id" 2>/dev/null || true
sleep 0.8

# ── [2/5] Preserve previous logs ──────────────────────────────────────────────
echo "[2/5] Preserving previous logs as *.last.log ..."
for i in 1 2 3 4; do
    log="$SCRIPT_DIR/node${i}.log"
    if [[ -f "$log" ]]; then
        cp -f "$log" "$SCRIPT_DIR/node${i}.last.log"
    fi
done

# ── [2b] Clear checkpoints (fresh start) ──────────────────────────────────────
if [[ "$KEEP_CHECKPOINTS" == true ]]; then
    echo "[2b] Keeping existing checkpoints (--keep-checkpoints set)."
else
    echo "[2b] Clearing checkpoints/ for a clean restart..."
    rm -rf "$SCRIPT_DIR/checkpoints"
fi

# ── [3/5] Clean old binary and rebuild ────────────────────────────────────────
echo "[3/5] Removing stale executables and rebuilding..."
rm -f "$SCRIPT_DIR/auction_node" "$SCRIPT_DIR/auction_node.exe" "$SCRIPT_DIR/distributed-auction.exe"

go build -o auction_node .
echo "    Build successful → $EXE"

# ── [4/5] Start 4 nodes in the background ─────────────────────────────────────
echo "[4/5] Starting 4 nodes..."
declare -a PIDS=()

for i in 1 2 3 4; do
    port=$((8000 + i))
    log="$SCRIPT_DIR/node${i}.log"
    "$EXE" --id="Node${i}" --port="$port" --host=0.0.0.0 --peers="$PEERS" \
        >"$log" 2>&1 &
    PIDS+=($!)
    echo "    Node${i} started  (port=$port  pid=${PIDS[-1]}  log=node${i}.log)"
done

sleep 2

# ── [5/5] Process summary ─────────────────────────────────────────────────────
echo "[5/5] Running process summary:"
printf "  %-8s %-10s %s\n" "PID" "NodeID" "Port"
printf "  %-8s %-10s %s\n" "---" "------" "----"
for i in "${!PIDS[@]}"; do
    node_num=$((i + 1))
    pid="${PIDS[$i]}"
    port=$((8000 + node_num))
    if kill -0 "$pid" 2>/dev/null; then
        printf "  %-8s %-10s %s\n" "$pid" "Node${node_num}" "$port"
    else
        printf "  %-8s %-10s %s  [EXITED]\n" "$pid" "Node${node_num}" "$port"
    fi
done

echo ""
echo "Done. Previous logs are available as node1.last.log ... node4.last.log"
echo "Logs: node1.log  node2.log  node3.log  node4.log"
echo "To stop all nodes: pkill -f 'auction_node --id'"
