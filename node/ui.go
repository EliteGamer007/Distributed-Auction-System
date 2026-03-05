package node

// ui.go — Serves the single-page auction UI over HTTP.

import (
	"fmt"
	"net/http"
)

func (n *Node) handleUI(w http.ResponseWriter, r *http.Request) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Auction — %s</title>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&display=swap" rel="stylesheet">
  <style>
    :root {
      --bg: #000000;
      --surface: rgba(28, 28, 30, 0.6);
      --surface2: rgba(44, 44, 46, 0.4);
      --border: rgba(255, 255, 255, 0.1);
      --text: #ffffff;
      --muted: #8e8e93;
      --accent: #ffffff;
      --accent2: #f2f2f7;
      --green: #34c759;
      --yellow: #ffcc00;
      --red: #ff3b30;
      --gold: #ffcc00;
      --transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
    }
    * { margin:0; padding:0; box-sizing:border-box; -webkit-font-smoothing: antialiased; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Inter', sans-serif;
      background: var(--bg);
      color: var(--text);
      min-height: 100vh;
      display: flex;
      flex-direction: column;
      align-items: center;
      padding: 48px 24px 80px;
      line-height: 1.5;
      background-image: radial-gradient(circle at 50%% -20%%, #1c1c1e 0%%, #000000 100%%);
    }
    header {
      width: 100%%; max-width: 1000px;
      display: flex; justify-content: space-between; align-items: center;
      margin-bottom: 48px;
      padding-bottom: 24px;
      border-bottom: 0.5px solid var(--border);
    }
    .brand { display: flex; align-items: center; gap: 12px; }
    .logo { font-size: 1.5rem; font-weight: 600; letter-spacing: -0.02em; color: white; }
    .node-info { display: flex; align-items: center; gap: 12px; }
    .leader-badge {
      display: none;
      font-size: 0.7rem; font-weight: 600;
      background: rgba(52, 199, 89, 0.1);
      border: 0.5px solid var(--green);
      color: var(--green);
      border-radius: 6px; padding: 4px 12px;
      text-transform: uppercase; letter-spacing: 0.05em;
      backdrop-filter: blur(20px); -webkit-backdrop-filter: blur(20px);
    }
    .layout {
      width: 100%%; max-width: 1000px;
      display: grid; grid-template-columns: 1fr 360px; gap: 32px;
    }
    @media (max-width: 900px) { .layout { grid-template-columns: 1fr; } }

    .current-card {
      background: linear-gradient(135deg, rgba(255, 255, 255, 0.08) 0%%, rgba(255, 255, 255, 0) 40%%), var(--surface);
      border: 0.5px solid rgba(255, 255, 255, 0.2);
      border-radius: 20px; padding: 40px;
      display: flex; flex-direction: column; gap: 32px;
      backdrop-filter: blur(40px); -webkit-backdrop-filter: blur(40px);
      box-shadow: 0 0 20px rgba(255, 255, 255, 0.05), 0 20px 50px rgba(0, 0, 0, 0.6);
      position: relative; overflow: hidden;
    }
    .current-card::after {
      content: ""; position: absolute; top: 0; left: 0; right: 0; height: 1px;
      background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.4), transparent);
    }
    .item-header { border-bottom: 0.5px solid var(--border); padding-bottom: 24px; }
    .item-name { font-size: 2.5rem; font-weight: 700; letter-spacing: -0.04em; color: white; }
    .item-desc { font-size: 1.125rem; color: var(--muted); margin-top: 8px; font-weight: 400; }

    .countdown-wrap { display: flex; flex-direction: column; gap: 12px; }
    .countdown-label { font-size: 0.75rem; font-weight: 600; color: var(--muted); letter-spacing: 0.08em; text-transform: uppercase; }
    .countdown {
      font-size: 4rem; font-weight: 700; font-variant-numeric: tabular-nums; letter-spacing: -0.02em;
    }
    .countdown.green { color: white; }
    .countdown.yellow { color: var(--yellow); }
    .countdown.red { color: var(--red); }

    .progress-bar-wrap { height: 4px; background: var(--surface2); border-radius: 2px; overflow: hidden; margin-top: 8px; }
    .progress-bar { height: 100%%; border-radius: 2px; transition: width 1s linear, background 0.3s; }

    .bid-info { display: flex; gap: 48px; padding: 24px 0; }
    .stat { display: flex; flex-direction: column; gap: 8px; }
    .stat-label { font-size: 0.75rem; font-weight: 600; color: var(--muted); text-transform: uppercase; letter-spacing: 0.08em; }
    .stat-value { font-size: 2rem; font-weight: 700; letter-spacing: -0.02em; }
    .stat-value.money { color: white; }
    .stat-value.winner { color: white; }

    .bid-form { display: flex; flex-direction: column; gap: 20px; margin-top: 12px; }
    .input-row { display: flex; gap: 12px; }
    input[type=text], input[type=number] {
      flex: 1; padding: 14px 20px;
      background: rgba(255, 255, 255, 0.05); border: 0.5px solid var(--border);
      border-radius: 12px; color: white;
      font-size: 1rem; font-family: inherit; outline: none;
      transition: var(--transition);
    }
    input[type=text]:focus, input[type=number]:focus {
      background: rgba(255, 255, 255, 0.08); border-color: rgba(255, 255, 255, 0.3);
    }
    .btn {
      padding: 14px 28px; border-radius: 12px; border: none; cursor: pointer;
      font-size: 1rem; font-weight: 600; font-family: inherit;
      background: #ffffff; color: #000000; transition: var(--transition);
      box-shadow: 0 4px 15px rgba(255, 255, 255, 0.1);
    }
    .btn:hover { background: #f2f2f7; transform: scale(1.02); box-shadow: 0 6px 20px rgba(255, 255, 255, 0.2); }
    .btn:active { transform: scale(0.98); }
    .btn:disabled { opacity: 0.3; cursor: not-allowed; transform: none; }
    #feedback { font-size: 0.9rem; font-weight: 500; min-height: 20px; text-align: center; }
    .err { color: var(--red); } .ok { color: var(--green); }

    .ended-banner {
      text-align: center; padding: 64px;
      background: linear-gradient(135deg, rgba(255, 255, 255, 0.05), transparent), var(--surface);
      border: 0.5px solid rgba(255, 255, 255, 0.2);
      border-radius: 20px; color: white;
      font-size: 1.75rem; font-weight: 700;
      backdrop-filter: blur(40px); -webkit-backdrop-filter: blur(40px);
      box-shadow: 0 10px 40px rgba(0, 0, 0, 0.6);
    }

    .sidebar { display: flex; flex-direction: column; gap: 32px; }
    .panel { 
      background: linear-gradient(135deg, rgba(255, 255, 255, 0.05), transparent), var(--surface);
      border: 0.5px solid rgba(255, 255, 255, 0.2);
      border-radius: 20px; padding: 28px; 
      backdrop-filter: blur(40px); -webkit-backdrop-filter: blur(40px);
      box-shadow: 0 0 20px rgba(255, 255, 255, 0.03), 0 10px 30px rgba(0, 0, 0, 0.4);
    }
    .panel-title { 
      font-size: 0.75rem; font-weight: 700; color: var(--muted); 
      text-transform: uppercase; letter-spacing: 0.1em; 
      margin-bottom: 24px; border-bottom: 0.5px solid var(--border); 
      padding-bottom: 12px; 
    }

    .queue-list, .results-list { display: flex; flex-direction: column; gap: 16px; }
    .item-row { 
      display: flex; justify-content: space-between; align-items: center; 
      padding: 16px; border-radius: 12px; 
      background: rgba(255, 255, 255, 0.03); border: 0.5px solid transparent; 
      transition: var(--transition);
    }
    .item-row:hover { background: rgba(255, 255, 255, 0.06); border-color: var(--border); }
    .item-info { flex: 1; }
    .item-row-title { font-size: 1rem; font-weight: 600; color: white; }
    .item-row-meta { font-size: 0.85rem; color: var(--muted); margin-top: 4px; }
    .item-row-side { font-size: 0.95rem; font-weight: 600; color: white; }

    .cp-row { display: flex; justify-content: space-between; align-items: center; padding: 10px 0; }
    .cp-key { font-size: 0.8rem; color: var(--muted); }
    .cp-val { font-size: 0.85rem; font-weight: 500; color: white; }
    .cp-dot { display: inline-block; width: 6px; height: 6px; border-radius: 50%; margin-right: 8px; background: var(--green); }
    .cp-dot.stale { background: var(--yellow); }
    .cp-dot.none { background: var(--border); }

    .admin-form { display: flex; flex-direction: column; gap: 16px; }
    .btn.secondary { background: rgba(255, 255, 255, 0.1); color: white; border: 0.5px solid var(--border); }
    .btn.secondary:hover { background: rgba(255, 255, 255, 0.15); }
    .btn.small { padding: 10px 20px; font-size: 0.9rem; }
    .empty-state { color: var(--muted); font-size: 0.9rem; text-align: center; padding: 24px 0; }

  </style>
</head>
<body>
<header>
  <div class="brand">
    <div class="logo">Auction House</div>
  </div>
  <div class="node-info">
    <div id="leaderBadge" class="leader-badge">Leader</div>
  </div>
</header>

<div class="layout">
  <div id="mainCol">
    <div id="currentCard" class="current-card">
      <div class="item-header">
        <div class="item-name" id="itemName">Loading…</div>
        <div class="item-desc" id="itemDesc"></div>
      </div>
      <div class="countdown-wrap">
        <div class="countdown-label">Time Remaining</div>
        <div class="countdown" id="countdown">--:--</div>
        <div class="progress-bar-wrap">
          <div class="progress-bar" id="progressBar" style="width:100%%; background:var(--green);"></div>
        </div>
      </div>
      <div class="bid-info">
        <div class="stat">
          <div class="stat-label">Highest Bid</div>
          <div class="stat-value money" id="highestBid">$0</div>
        </div>
        <div class="stat">
          <div class="stat-label">Leading Bidder</div>
          <div class="stat-value winner" id="winner">—</div>
        </div>
      </div>
      <div class="bid-form">
        <div class="input-row">
          <input type="text" id="bidderName" placeholder="Your Name" autocomplete="off">
          <input type="number" id="amount" placeholder="Bid Amount ($)" min="1" autocomplete="off">
          <button class="btn" id="bidBtn" onclick="submitBid()">Place Bid</button>
        </div>
        <div id="feedback"></div>
      </div>
    </div>

    <div class="panel" id="adminPanel" style="margin-top:24px; display:none;">
      <div class="panel-title">Admin Controls</div>
      <div class="admin-form">
        <input type="text" id="newItemName" placeholder="New Item Name" autocomplete="off">
        <input type="text" id="newItemDesc" placeholder="Description" autocomplete="off">
        <div class="input-row">
          <input type="number" id="newItemPrice" placeholder="Starting Price ($)" min="1" autocomplete="off">
          <input type="number" id="newItemDuration" placeholder="Duration (sec)" min="10" autocomplete="off">
        </div>
        <button class="btn small" id="addItemBtn" onclick="addItem()">Add to Queue</button>
        <div style="display:flex; gap:8px;">
          <button class="btn secondary small" id="startAuctionBtn" onclick="auctionControl('start')">Start</button>
          <button class="btn secondary small" id="stopAuctionBtn" onclick="auctionControl('stop')">Stop</button>
          <button class="btn secondary small" id="restartAuctionBtn" onclick="auctionControl('restart')">Restart</button>
        </div>
        <div id="adminFeedback" class="admin-feedback"></div>
      </div>
    </div>

    <div id="endedBanner" class="ended-banner" style="display:none">
      Auction Complete — All items sold
    </div>
  </div>

  <div class="sidebar">
    <div class="panel">
      <div class="panel-title">Up Next</div>
      <div class="queue-list" id="queueList"><div class="empty-state">Queue loading…</div></div>
    </div>
    <div class="panel">
      <div class="panel-title">Sold</div>
      <div class="results-list" id="resultsList"><div class="empty-state">No items sold yet</div></div>
    </div>
    <div class="panel">
      <div class="panel-title">Checkpoint</div>
      <div id="cpPanel">
        <div class="cp-row"><span class="cp-key">Status</span><span class="cp-val" id="cpStatus"><span class="cp-dot none"></span>None</span></div>
        <div class="cp-row"><span class="cp-key">Saved at</span><span class="cp-val" id="cpTime">—</span></div>
        <div class="cp-row"><span class="cp-key">Lamport</span><span class="cp-val" id="cpLamport">—</span></div>
        <div class="cp-row"><span class="cp-key">Results saved</span><span class="cp-val" id="cpResults">—</span></div>
      </div>
    </div>
  </div>
</div>

<script>
  let totalDuration = 60;
  let deadlineUnix = 0;
  let localTimerInterval = null;

  function fmt2(n){ return String(n).padStart(2,'0'); }

  function startLocalTimer(deadline, duration) {
    deadlineUnix = deadline;
    totalDuration = duration || 60;
    if (localTimerInterval) clearInterval(localTimerInterval);
    localTimerInterval = setInterval(tickTimer, 250);
    tickTimer();
  }

  function tickTimer() {
    const now = Math.floor(Date.now() / 1000);
    const remaining = Math.max(0, deadlineUnix - now);
    const mins = Math.floor(remaining / 60);
    const secs = remaining %% 60;
    const el = document.getElementById('countdown');
    el.textContent = fmt2(mins) + ':' + fmt2(secs);

    const fraction = totalDuration > 0 ? remaining / totalDuration : 0;
    const bar = document.getElementById('progressBar');
    bar.style.width = (fraction * 100) + '%%';

    el.className = 'countdown';
    if (remaining > totalDuration * 0.5) {
      el.classList.add('green'); bar.style.background = 'var(--green)';
    } else if (remaining > totalDuration * 0.2) {
      el.classList.add('yellow'); bar.style.background = 'var(--yellow)';
    } else {
      el.classList.add('red'); bar.style.background = 'var(--red)';
    }

    if (remaining === 0 && localTimerInterval) {
      clearInterval(localTimerInterval);
      localTimerInterval = null;
      document.getElementById('countdown').textContent = '00:00';
    }
  }

  async function fetchState() {
    try {
      const res = await fetch('/state');
      const d = await res.json();
      // Admin panel always visible - actions proxy to coordinator
      document.getElementById('adminPanel').style.display = 'block';

      if (!d.Active || !d.CurrentItem) {
        document.getElementById('currentCard').style.display = 'none';
        document.getElementById('endedBanner').style.display = 'block';
        if (localTimerInterval) { clearInterval(localTimerInterval); localTimerInterval = null; }
        renderQueue([]);
        renderResults(d.Results || []);
        return;
      }

      document.getElementById('currentCard').style.display = 'flex';
      document.getElementById('endedBanner').style.display = 'none';

      const item = d.CurrentItem;
      document.getElementById('itemName').textContent = item.Name;
      document.getElementById('itemDesc').textContent = item.Description;
      document.getElementById('highestBid').textContent = '$' + d.CurrentHighestBid;
      document.getElementById('winner').textContent = d.CurrentWinner || '—';

      // Leader indicator
      document.getElementById('leaderBadge').style.display = d.IsCoordinator ? 'inline-block' : 'none';

      if (d.DeadlineUnix && d.DeadlineUnix !== deadlineUnix) {
        startLocalTimer(d.DeadlineUnix, item.DurationSec);
      }

      renderQueue(d.RemainingItems || []);
      renderResults(d.Results || []);
    } catch(e) { console.error('state fetch error', e); }
  }

  function renderQueue(items) {
    const el = document.getElementById('queueList');
    if (!items.length) { el.innerHTML = '<div class="empty-state">No more items</div>'; return; }
    el.innerHTML = items.map(function(it) {
      return '<div class="item-row">' +
        '<div class="item-info">' +
          '<div class="item-row-title">' + it.Name + '</div>' +
          '<div class="item-row-meta">' + it.Description + '</div>' +
        '</div>' +
        '<div class="item-row-side">$' + it.StartingPrice + '</div>' +
        '</div>';
    }).join('');
  }

  function renderResults(results) {
    const el = document.getElementById('resultsList');
    if (!results.length) { el.innerHTML = '<div class="empty-state">No items sold yet</div>'; return; }
    el.innerHTML = [...results].reverse().map(function(r) {
      var winnerText = r.Winner === 'No bids' ? 'Unsold' : ('Won by ' + r.Winner);
      var bidText = r.WinningBid > 0 ? ('$' + r.WinningBid) : '\u2014';
      return '<div class="item-row">' +
        '<div class="item-info">' +
          '<div class="item-row-title">' + r.Item.Name + '</div>' +
          '<div class="item-row-meta">' + winnerText + '</div>' +
        '</div>' +
        '<div class="item-row-side">' + bidText + '</div>' +
      '</div>';
    }).join('');
  }

  async function submitBid() {
    const amount = document.getElementById('amount').value;
    const bidder = document.getElementById('bidderName').value.trim() || 'Anonymous';
    const fb = document.getElementById('feedback');
    const btn = document.getElementById('bidBtn');
    if (!amount) { fb.textContent = 'Enter a bid amount'; fb.className = 'err'; return; }

    btn.disabled = true;
    fb.className = ''; fb.textContent = 'Submitting…';

    const body = new URLSearchParams();
    body.append('amount', amount);
    body.append('bidder', bidder);

    try {
      const res = await fetch('/bid', { method:'POST', body, headers:{'Content-Type':'application/x-www-form-urlencoded'} });
      if (!res.ok) {
        fb.textContent = await res.text(); fb.className = 'err';
        setTimeout(function() { fb.textContent = ''; fb.className = ''; }, 10000);
      } else {
        fb.textContent = await res.text(); fb.className = 'ok';
        document.getElementById('amount').value = '';
        setTimeout(function() { fb.textContent = ''; }, 3000);
        fetchState();
      }
    } catch(e) {
      fb.textContent = 'Network error. Try again.'; fb.className = 'err';
    }
    btn.disabled = false;
  }

  async function fetchCheckpoint() {
    try {
      const res = await fetch('/checkpoint');
      if (res.status === 404) {
        document.getElementById('cpStatus').innerHTML = '<span class="cp-dot none"></span>None yet';
        document.getElementById('cpTime').textContent = '—';
        document.getElementById('cpLamport').textContent = '—';
        document.getElementById('cpResults').textContent = '—';
        return;
      }
      const d = await res.json();
      const savedAt = new Date(d.checkpointTime * 1000);
      const ageS = Math.floor((Date.now() / 1000) - d.checkpointTime);
      const fresh = ageS < 60;
      document.getElementById('cpStatus').innerHTML = '<span class="cp-dot' + (fresh ? '' : ' stale') + '"></span>' + (fresh ? 'Fresh' : 'Stale (' + ageS + 's ago)');
      document.getElementById('cpTime').textContent = savedAt.toLocaleTimeString();
      document.getElementById('cpLamport').textContent = d.lamportStamp;
      document.getElementById('cpResults').textContent = (d.results ? d.results.length : 0) + ' items';
    } catch(e) { console.error('checkpoint fetch error', e); }
  }

  async function addItem() {
    const name = document.getElementById('newItemName').value.trim();
    const description = document.getElementById('newItemDesc').value.trim();
    const startingPrice = document.getElementById('newItemPrice').value;
    const durationSec = document.getElementById('newItemDuration').value;
    const fb = document.getElementById('adminFeedback');
    const btn = document.getElementById('addItemBtn');

    if (!name || !description || !startingPrice || !durationSec) {
      fb.textContent = 'Fill all item fields';
      fb.className = 'admin-feedback err';
      return;
    }

    const body = new URLSearchParams();
    body.append('name', name);
    body.append('description', description);
    body.append('startingPrice', startingPrice);
    body.append('durationSec', durationSec);

    btn.disabled = true;
    fb.textContent = 'Submitting…';
    fb.className = 'admin-feedback';

    try {
      const res = await fetch('/admin/item', {
        method: 'POST',
        body,
        headers: {'Content-Type': 'application/x-www-form-urlencoded'}
      });
      const msg = await res.text();
      if (!res.ok) {
        fb.textContent = msg;
        fb.className = 'admin-feedback err';
      } else {
        fb.textContent = msg;
        fb.className = 'admin-feedback ok';
        document.getElementById('newItemName').value = '';
        document.getElementById('newItemDesc').value = '';
        document.getElementById('newItemPrice').value = '';
        document.getElementById('newItemDuration').value = '';
        fetchState();
      }
    } catch (e) {
      fb.textContent = 'Network error. Try again.';
      fb.className = 'admin-feedback err';
    }
    btn.disabled = false;
  }

  async function auctionControl(action) {
    const fb = document.getElementById('adminFeedback');
    const startBtn = document.getElementById('startAuctionBtn');
    const stopBtn = document.getElementById('stopAuctionBtn');
    const restartBtn = document.getElementById('restartAuctionBtn');
    startBtn.disabled = true;
    stopBtn.disabled = true;
    restartBtn.disabled = true;
    fb.textContent = action === 'start' ? 'Starting…' : (action === 'stop' ? 'Stopping…' : 'Restarting…');
    fb.className = 'admin-feedback';

    const body = new URLSearchParams();
    body.append('action', action);

    try {
      const res = await fetch('/admin/auction', {
        method: 'POST',
        body,
        headers: {'Content-Type': 'application/x-www-form-urlencoded'}
      });
      const msg = await res.text();
      if (!res.ok) {
        fb.textContent = msg;
        fb.className = 'admin-feedback err';
      } else {
        fb.textContent = msg;
        fb.className = 'admin-feedback ok';
        fetchState();
      }
    } catch (e) {
      fb.textContent = 'Network error. Try again.';
      fb.className = 'admin-feedback err';
    }

    startBtn.disabled = false;
    stopBtn.disabled = false;
    restartBtn.disabled = false;
  }

  setInterval(fetchState, 1000);
  setInterval(fetchCheckpoint, 15000);
  fetchState();
  fetchCheckpoint();
</script>
</body>
</html>`, n.ID)

	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write([]byte(html))
}
