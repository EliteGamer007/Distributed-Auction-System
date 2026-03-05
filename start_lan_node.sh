#!/bin/bash
# start_lan_node.sh — Launch a single auction node for LAN deployment.
# Usage:  chmod +x start_lan_node.sh && ./start_lan_node.sh

# Exit on error
set -e

# Get the directory of the script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# ── CONFIG: change MyNodeId per laptop ──────────────────────────────────
MyNodeId=4       # <-- Set to 1, 2, 3, or 4 depending on the laptop

declare -A LanIPs
LanIPs[1]='10.12.234.8'
LanIPs[2]='10.12.228.13'
LanIPs[3]='10.12.226.20'
LanIPs[4]='10.12.227.210'

Port=$((8000 + MyNodeId))   # Node1→8001, Node2→8002, etc.
# ─────────────────────────────────────────────────────────────────────────

echo "============================================"
echo "  Distributed Auction System - LAN Mode"
myIP=${LanIPs[$MyNodeId]}
echo "  This laptop: Node$MyNodeId ($myIP:$Port)"
echo "============================================"

# [1] Stop old processes
echo -e "\n[1/4] Stopping old auction_node processes..."
# Find processes named auction_node that are running from this directory
# We look for the executable in the current directory specifically to match PS1 logic
pgrep -f "$(pwd)/auction_node" | xargs -r kill -9 || true
sleep 0.8

# [2] Preserve previous log
echo "[2/4] Preserving previous log..."
log="node${MyNodeId}.log"
lastLog="node${MyNodeId}.last.log"
if [ -f "$log" ]; then
    cp "$log" "$lastLog"
fi

# [3] Rebuild
echo "[3/4] Rebuilding auction_node..."
exePath="./auction_node"
if [ -f "$exePath" ]; then
    rm "$exePath"
fi
go build -o auction_node .

# [4] Build peer list (all nodes except self)
peers=()
for id in {1..4}; do
    if [ "$id" -ne "$MyNodeId" ]; then
        peerPort=$((8000 + id))
        peers+=("${LanIPs[$id]}:$peerPort")
    fi
done

# Join peers with comma
peerList=$(IFS=,; echo "${peers[*]}")

echo "[4/4] Starting Node$MyNodeId on 0.0.0.0:$Port"
echo "      Peers: $peerList"
echo "      UI:    http://localhost:$Port"
echo "      LAN:   http://$myIP:$Port"
echo ""
echo "Press Ctrl+C to stop."
echo "--------------------------------------------"

# Run in foreground
nodeId="Node$MyNodeId"
$exePath --id "$nodeId" --host 0.0.0.0 --port "$Port" --peers "$peerList"
