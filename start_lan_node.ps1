# start_lan_node.ps1 — Launch a single auction node for LAN deployment.
# Usage:  powershell -ExecutionPolicy Bypass -File .\start_lan_node.ps1
#
# Edit the two variables below to match THIS laptop's role:
#   $MyNodeId  — which node number this laptop runs (1-4)
#   $LanIPs    — the IP addresses of ALL 4 laptops on Wi-Fi
#
# Node mapping:
#   Node1 → 10.154.195.215
#   Node2 → 10.154.195.247
#   Node3 → 10.154.195.104   ← this laptop
#   Node4 → 10.154.195.155

$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

# ── CONFIG: change $MyNodeId per laptop ──────────────────────────────────
$MyNodeId = 3       # <-- Set to 1, 2, 3, or 4 depending on the laptop

$LanIPs = @{
    1 = '10.154.195.215'
    2 = '10.154.195.247'
    3 = '10.154.195.104'
    4 = '10.154.195.155'
}
$Port = 8000 + $MyNodeId   # Node1→8001, Node2→8002, etc.
# ─────────────────────────────────────────────────────────────────────────

Write-Host "============================================"
Write-Host "  Distributed Auction System - LAN Mode"
$myIP = $LanIPs[$MyNodeId]
Write-Host ("  This laptop: Node{0} ({1}:{2})" -f $MyNodeId, $myIP, $Port)
Write-Host "============================================"

# [1] Stop old processes
Write-Host "`n[1/4] Stopping old auction_node processes..."
Get-CimInstance Win32_Process |
    Where-Object {
        ($_.Name -eq 'auction_node.exe') -and
        $_.ExecutablePath -and
        $_.ExecutablePath.StartsWith($PSScriptRoot, [System.StringComparison]::OrdinalIgnoreCase)
    } |
    ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
Start-Sleep -Milliseconds 800

# [2] Preserve previous log
Write-Host "[2/4] Preserving previous log..."
$log = Join-Path $PSScriptRoot ('node{0}.log' -f $MyNodeId)
$lastLog = Join-Path $PSScriptRoot ('node{0}.last.log' -f $MyNodeId)
if (Test-Path $log) { Copy-Item $log $lastLog -Force }

# [3] Rebuild
Write-Host "[3/4] Rebuilding auction_node.exe..."
$exePath = Join-Path $PSScriptRoot 'auction_node.exe'
if (Test-Path $exePath) { Remove-Item $exePath -Force }
go build -o auction_node.exe .
if ($LASTEXITCODE -ne 0) { throw "Build failed" }

# [4] Build peer list (all nodes except self)
$peers = @()
foreach ($id in 1..4) {
    if ($id -ne $MyNodeId) {
        $peerPort = 8000 + $id
        $peers += ('{0}:{1}' -f $LanIPs[$id], $peerPort)
    }
}
$peerList = $peers -join ','

Write-Host ('[4/4] Starting Node{0} on 0.0.0.0:{1}' -f $MyNodeId, $Port)
Write-Host ('      Peers: {0}' -f $peerList)
Write-Host ('      UI:    http://localhost:{0}' -f $Port)
Write-Host ('      LAN:   http://{0}:{1}' -f $myIP, $Port)
Write-Host ""
Write-Host "Press Ctrl+C to stop."
Write-Host "--------------------------------------------"

# Run in foreground so you see live output (Ctrl+C to stop)
$nodeId = 'Node{0}' -f $MyNodeId
& $exePath --id $nodeId --host 0.0.0.0 --port $Port --peers $peerList
