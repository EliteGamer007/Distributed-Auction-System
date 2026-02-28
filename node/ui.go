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
      --bg: #0a0a0f;
      --surface: #13131a;
      --surface2: #1c1c26;
      --border: rgba(255,255,255,0.08);
      --text: #f0f0f5;
      --muted: #6b6b80;
      --accent: #7c6aff;
      --accent2: #a78bfa;
      --green: #22c55e;
      --yellow: #eab308;
      --red: #ef4444;
      --gold: #f59e0b;
      --transition: all 0.25s cubic-bezier(0.4,0,0.2,1);
    }
    * { margin:0; padding:0; box-sizing:border-box; }
    body {
      font-family: 'Inter', sans-serif;
      background: var(--bg);
      color: var(--text);
      min-height: 100vh;
      display: flex;
      flex-direction: column;
      align-items: center;
      padding: 24px 16px 48px;
      -webkit-font-smoothing: antialiased;
    }
    header {
      width: 100%%; max-width: 900px;
      display: flex; justify-content: space-between; align-items: center;
      margin-bottom: 32px;
      padding-bottom: 16px;
      border-bottom: 1px solid var(--border);
    }
    .logo { font-size: 1.2rem; font-weight: 700; letter-spacing: -0.02em; }
    .logo span { color: var(--accent2); }
    .node-badge {
      font-size: 0.75rem; font-weight: 600;
      background: var(--surface2); border: 1px solid var(--border);
      border-radius: 999px; padding: 4px 14px;
      color: var(--muted); letter-spacing: 0.08em; text-transform: uppercase;
    }
    .layout {
      width: 100%%; max-width: 900px;
      display: grid; grid-template-columns: 1fr 320px; gap: 20px;
    }
    @media (max-width: 700px) { .layout { grid-template-columns: 1fr; } }

    /* Current Item Card */
    .current-card {
      background: linear-gradient(135deg, #1a1730 0%%, #13131a 60%%);
      border: 1px solid rgba(124,106,255,0.25);
      border-radius: 24px; padding: 32px;
      display: flex; flex-direction: column; gap: 24px;
      box-shadow: 0 0 60px rgba(124,106,255,0.08);
    }
    .item-emoji { font-size: 4rem; line-height: 1; margin-bottom: 4px; }
    .item-name { font-size: 1.8rem; font-weight: 700; letter-spacing: -0.03em; }
    .item-desc { font-size: 0.95rem; color: var(--muted); margin-top: 4px; }

    /* Countdown */
    .countdown-wrap { display: flex; flex-direction: column; gap: 8px; }
    .countdown-label { font-size: 0.75rem; font-weight: 600; color: var(--muted); letter-spacing: 0.1em; text-transform: uppercase; }
    .countdown {
      font-size: 3.5rem; font-weight: 700; letter-spacing: -0.04em;
      font-variant-numeric: tabular-nums; transition: color 0.5s;
    }
    .countdown.green { color: var(--green); }
    .countdown.yellow { color: var(--yellow); }
    .countdown.red { color: var(--red); animation: pulse 0.8s infinite; }
    @keyframes pulse { 0%%,100%% { opacity:1; } 50%% { opacity:0.5; } }

    .progress-bar-wrap { height: 4px; background: var(--surface2); border-radius: 2px; overflow: hidden; }
    .progress-bar { height: 100%%; border-radius: 2px; transition: width 0.9s linear, background 0.5s; }

    /* Bid info */
    .bid-info { display: flex; gap: 20px; }
    .stat { display: flex; flex-direction: column; gap: 4px; }
    .stat-label { font-size: 0.72rem; font-weight: 600; color: var(--muted); text-transform: uppercase; letter-spacing: 0.08em; }
    .stat-value { font-size: 1.5rem; font-weight: 700; }
    .stat-value.money { color: var(--accent2); }
    .stat-value.winner { font-size: 1.1rem; color: var(--text); }

    /* Bid form */
    .bid-form { display: flex; flex-direction: column; gap: 12px; }
    .input-row { display: flex; gap: 10px; }
    input[type=text], input[type=number] {
      flex: 1; padding: 14px 18px;
      background: var(--surface2); border: 1px solid var(--border);
      border-radius: 14px; color: var(--text);
      font-size: 1rem; font-family: inherit; outline: none;
      transition: var(--transition);
    }
    input[type=text]:focus, input[type=number]:focus {
      border-color: var(--accent); box-shadow: 0 0 0 3px rgba(124,106,255,0.15);
    }
    input::-webkit-outer-spin-button, input::-webkit-inner-spin-button { -webkit-appearance: none; }
    .btn {
      padding: 14px 28px; border-radius: 14px; border: none; cursor: pointer;
      font-size: 1rem; font-weight: 600; font-family: inherit;
      background: linear-gradient(135deg, var(--accent), var(--accent2));
      color: white; transition: var(--transition); white-space: nowrap;
    }
    .btn:hover { transform: translateY(-1px); box-shadow: 0 8px 24px rgba(124,106,255,0.35); }
    .btn:active { transform: translateY(0); }
    .btn:disabled { opacity: 0.4; cursor: not-allowed; transform: none; box-shadow: none; }
    #feedback { font-size: 0.88rem; font-weight: 500; min-height: 20px; transition: var(--transition); }
    .err { color: var(--red); } .ok { color: var(--green); }

    /* Auction ended */
    .ended-banner {
      text-align: center; padding: 32px;
      background: linear-gradient(135deg, #1a1a10, #13131a);
      border: 1px solid rgba(245,158,11,0.25);
      border-radius: 24px; color: var(--gold);
      font-size: 1.5rem; font-weight: 700;
    }

    /* Sidebar */
    .sidebar { display: flex; flex-direction: column; gap: 20px; }
    .panel { background: var(--surface); border: 1px solid var(--border); border-radius: 20px; padding: 20px; }
    .panel-title { font-size: 0.72rem; font-weight: 700; color: var(--muted); text-transform: uppercase; letter-spacing: 0.1em; margin-bottom: 16px; }

    .queue-list { display: flex; flex-direction: column; gap: 10px; }
    .queue-item { display: flex; align-items: center; gap: 12px; padding: 10px 12px; border-radius: 12px; background: var(--surface2); }
    .queue-emoji { font-size: 1.4rem; }
    .queue-info { flex: 1; min-width: 0; }
    .queue-name { font-size: 0.88rem; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
    .queue-start { font-size: 0.75rem; color: var(--muted); margin-top: 2px; }

    .results-list { display: flex; flex-direction: column; gap: 8px; }
    .result-item { display: flex; align-items: center; gap: 10px; padding: 10px 12px; border-radius: 12px; background: var(--surface2); border-left: 3px solid var(--gold); }
    .result-emoji { font-size: 1.2rem; }
    .result-info { flex: 1; min-width: 0; }
    .result-name { font-size: 0.82rem; font-weight: 600; }
    .result-winner { font-size: 0.75rem; color: var(--muted); margin-top: 2px; }
    .result-bid { font-size: 0.85rem; font-weight: 700; color: var(--gold); white-space: nowrap; }

    .empty-state { color: var(--muted); font-size: 0.85rem; text-align: center; padding: 12px 0; }

    /* Checkpoint panel */
    .cp-row { display: flex; justify-content: space-between; align-items: center; padding: 6px 0; border-bottom: 1px solid var(--border); }
    .cp-row:last-child { border-bottom: none; }
    .cp-key { font-size: 0.75rem; color: var(--muted); }
    .cp-val { font-size: 0.78rem; font-weight: 600; }
    .cp-dot { display: inline-block; width: 7px; height: 7px; border-radius: 50%%; margin-right: 5px; background: var(--green); }
    .cp-dot.stale { background: var(--yellow); }
    .cp-dot.none { background: var(--muted); }
  </style>
</head>
<body>
<header>
  <div class="logo">Auction<span>House</span></div>
  <div class="node-badge">%s</div>
</header>

<div class="layout">
  <div id="mainCol">
    <div id="currentCard" class="current-card">
      <div>
        <div class="item-emoji" id="itemEmoji">⏳</div>
        <div class="item-name" id="itemName">Loading…</div>
        <div class="item-desc" id="itemDesc"></div>
      </div>
      <div class="countdown-wrap">
        <div class="countdown-label">Time Remaining</div>
        <div class="countdown green" id="countdown">--:--</div>
        <div class="progress-bar-wrap">
          <div class="progress-bar" id="progressBar" style="width:100%%;background:var(--green);"></div>
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
          <input type="text" id="bidderName" placeholder="Your name" autocomplete="off">
          <input type="number" id="amount" placeholder="Bid amount ($)" min="1" autocomplete="off">
          <button class="btn" id="bidBtn" onclick="submitBid()">Bid</button>
        </div>
        <div id="feedback"></div>
      </div>
    </div>
    <div id="endedBanner" class="ended-banner" style="display:none">
      🎉 Auction Complete — All items sold!
    </div>
  </div>

  <div class="sidebar">
    <div class="panel">
      <div class="panel-title">⬇ Up Next</div>
      <div class="queue-list" id="queueList"><div class="empty-state">Queue loading…</div></div>
    </div>
    <div class="panel">
      <div class="panel-title">🏆 Sold</div>
      <div class="results-list" id="resultsList"><div class="empty-state">No items sold yet</div></div>
    </div>
    <div class="panel">
      <div class="panel-title">💾 Checkpoint</div>
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
      document.getElementById('itemEmoji').textContent = item.Emoji;
      document.getElementById('itemName').textContent = item.Name;
      document.getElementById('itemDesc').textContent = item.Description;
      document.getElementById('highestBid').textContent = '$' + d.CurrentHighestBid;
      document.getElementById('winner').textContent = d.CurrentWinner || '—';

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
      return '<div class="queue-item">' +
        '<div class="queue-emoji">' + it.Emoji + '</div>' +
        '<div class="queue-info">' +
          '<div class="queue-name">' + it.Name + '</div>' +
          '<div class="queue-start">Starting at $' + it.StartingPrice + '</div>' +
        '</div></div>';
    }).join('');
  }

  function renderResults(results) {
    const el = document.getElementById('resultsList');
    if (!results.length) { el.innerHTML = '<div class="empty-state">No items sold yet</div>'; return; }
    el.innerHTML = [...results].reverse().map(function(r) {
      var winnerText = r.Winner === 'No bids' ? 'Unsold' : ('Won by ' + r.Winner);
      var bidText = r.WinningBid > 0 ? ('$' + r.WinningBid) : '\u2014';
      return '<div class="result-item">' +
        '<div class="result-emoji">' + r.Item.Emoji + '</div>' +
        '<div class="result-info">' +
          '<div class="result-name">' + r.Item.Name + '</div>' +
          '<div class="result-winner">' + winnerText + '</div>' +
        '</div>' +
        '<div class="result-bid">' + bidText + '</div>' +
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

  setInterval(fetchState, 1000);
  setInterval(fetchCheckpoint, 15000);
  fetchState();
  fetchCheckpoint();
</script>
</body>
</html>`, n.ID, n.ID)

	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write([]byte(html))
}
