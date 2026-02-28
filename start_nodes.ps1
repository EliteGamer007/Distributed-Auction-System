$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

Write-Host "[1/5] Stopping old node processes..."
Get-CimInstance Win32_Process |
    Where-Object {
        ($_.Name -eq 'auction_node.exe' -or $_.Name -eq 'distributed-auction.exe') -and
        $_.ExecutablePath -and
        $_.ExecutablePath.StartsWith($PSScriptRoot, [System.StringComparison]::OrdinalIgnoreCase)
    } |
    ForEach-Object {
        Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue
    }

Start-Sleep -Milliseconds 800

Write-Host "[2/5] Preserving previous logs as *.last.log ..."
1..4 | ForEach-Object {
    $log = Join-Path $PSScriptRoot ("node{0}.log" -f $_)
    $lastLog = Join-Path $PSScriptRoot ("node{0}.last.log" -f $_)
    if (Test-Path $log) {
        Copy-Item $log $lastLog -Force
    }
}

Write-Host "[3/5] Removing stale executables and rebuilding..."
@('auction_node.exe', 'distributed-auction.exe') | ForEach-Object {
    $path = Join-Path $PSScriptRoot $_
    if (Test-Path $path) {
        Remove-Item $path -Force
    }
}

go build -o auction_node.exe .
if ($LASTEXITCODE -ne 0) {
    throw "Build failed"
}

$exe = Join-Path $PSScriptRoot 'auction_node.exe'
$peers = 'localhost:8001,localhost:8002,localhost:8003,localhost:8004'

Write-Host "[4/5] Starting 4 nodes..."
$nodes = @(
    @{ Id = '1'; Port = '8001'; Log = 'node1.log' },
    @{ Id = '2'; Port = '8002'; Log = 'node2.log' },
    @{ Id = '3'; Port = '8003'; Log = 'node3.log' },
    @{ Id = '4'; Port = '8004'; Log = 'node4.log' }
)

foreach ($node in $nodes) {
    $logPath = Join-Path $PSScriptRoot $node.Log
    $cmd = "& '$exe' --id=$($node.Id) --port=$($node.Port) --host=0.0.0.0 --peers=$peers *> '$logPath'"
    Start-Process -FilePath powershell.exe -WorkingDirectory $PSScriptRoot -WindowStyle Hidden -ArgumentList @(
        '-NoProfile',
        '-ExecutionPolicy', 'Bypass',
        '-Command',
        $cmd
    ) | Out-Null
}

Start-Sleep -Seconds 2

Write-Host "[5/5] Running process summary:"
Get-CimInstance Win32_Process -Filter "name='auction_node.exe'" |
    Where-Object { $_.ExecutablePath -eq $exe } |
    Select-Object ProcessId, ExecutablePath, CommandLine |
    Format-Table -AutoSize

Write-Host "Done. Previous logs are available as node1.last.log ... node4.last.log"
