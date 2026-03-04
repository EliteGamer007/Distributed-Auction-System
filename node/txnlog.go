package node

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const txnLogDir = "txlogs"

type TxnLogEntry struct {
	TimestampUnix int64  `json:"timestampUnix"`
	NodeID        string `json:"nodeId"`
	TxnID         string `json:"txnId"`
	Event         string `json:"event"`
	Message       string `json:"message"`
}

func txnLogPath(nodeID string) string {
	return filepath.Join(txnLogDir, fmt.Sprintf("txn_%s.log", nodeID))
}

func (n *Node) logTxnEvent(txnID, event, message string) {
	entry := TxnLogEntry{
		TimestampUnix: time.Now().Unix(),
		NodeID:        n.ID,
		TxnID:         txnID,
		Event:         event,
		Message:       message,
	}
	b, err := json.Marshal(entry)
	if err != nil {
		return
	}

	n.TxnLogMutex.Lock()
	defer n.TxnLogMutex.Unlock()

	if err := os.MkdirAll(txnLogDir, 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(txnLogPath(n.ID), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	_, _ = f.Write(append(b, '\n'))
}
