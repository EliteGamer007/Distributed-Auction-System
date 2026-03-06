package node

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// RunMonitor runs a live-refreshing terminal dashboard for the auction.
func RunMonitor() {
	fmt.Print("\033[H\033[2J") // Clear screen
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		state := fetchGlobalState()
		events := fetchRecentEvents(10)

		fmt.Print("\033[H") // Move cursor to top
		fmt.Println("================================================================")
		fmt.Println("                AUCTION SYSTEM LIVE MONITOR                     ")
		fmt.Println("================================================================")

		if state == nil {
			fmt.Println("\n  [!] Waiting for nodes to start...")
		} else {
			status := "INACTIVE"
			if state.Active {
				status = "ACTIVE"
			}
			fmt.Printf("\n  STATUS:  [%s]\n", status)

			if state.CurrentItem != nil {
				fmt.Printf("  ITEM:    %s\n", state.CurrentItem.Name)
				fmt.Printf("  BID:     $%d (by %s)\n", state.CurrentHighestBid, state.CurrentWinner)

				rem := state.DeadlineUnix - time.Now().Unix()
				if rem < 0 {
					rem = 0
				}
				fmt.Printf("  TIME:    %ds remaining\n", rem)
			} else {
				fmt.Println("  ITEM:    None")
			}
			fmt.Printf("\n  NODES:   Node1, Node2, Node3, Node4\n")
		}

		fmt.Println("\n----------------------- RECENT ACTIVITY -----------------------")
		if len(events) == 0 {
			fmt.Println("  (No activity logged yet)")
		} else {
			for _, e := range events {
				timeStr := time.Unix(e.TimestampUnix, 0).Format("15:04:05")
				fmt.Printf("  [%s] %-12s | %s\n", timeStr, e.Event, e.Message)
			}
		}
		fmt.Println("----------------------------------------------------------------")
		fmt.Println("\n  Press Ctrl+C to close monitor.")
	}
}

func fetchGlobalState() *QueueSnapshot {
	// Try nodes until one responds
	for i := 1; i <= 4; i++ {
		url := fmt.Sprintf("http://localhost:%d/state", 8000+i)
		resp, err := http.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var snap QueueSnapshot
		if err := json.Unmarshal(body, &snap); err == nil {
			return &snap
		}
	}
	return nil
}

func fetchRecentEvents(count int) []TxnLogEntry {
	files, _ := filepath.Glob("txlogs/*.log")
	var allEntries []TxnLogEntry

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			var entry TxnLogEntry
			if err := json.Unmarshal([]byte(line), &entry); err == nil {
				// Filter for interesting events
				if entry.Event == "TXN_COMMIT_APPLIED" || entry.Event == "TXN_BEGIN" ||
					strings.Contains(entry.Event, "AUCTION") || strings.Contains(entry.Event, "ITEM") {
					allEntries = append(allEntries, entry)
				}
			}
		}
	}

	sort.Slice(allEntries, func(i, j int) bool {
		return allEntries[i].TimestampUnix > allEntries[j].TimestampUnix
	})

	if len(allEntries) > count {
		return allEntries[:count]
	}
	return allEntries
}
