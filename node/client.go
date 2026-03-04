package node

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/rpc"
	"time"
)

const rpcDialTimeout = 3 * time.Second // fail fast for unreachable peers

type RPCClient struct{}

// dialHTTPTimeout is like rpc.DialHTTP but with a connect timeout so the
// system doesn't hang when peers are offline.
func dialHTTPTimeout(network, address string, timeout time.Duration) (*rpc.Client, error) {
	conn, err := net.DialTimeout(network, address, timeout)
	if err != nil {
		return nil, err
	}
	// Reproduce what rpc.DialHTTPPath does: send CONNECT, read response
	_, _ = io.WriteString(conn, "CONNECT "+rpc.DefaultRPCPath+" HTTP/1.0\n\n")
	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "CONNECT"})
	if err != nil {
		conn.Close()
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		conn.Close()
		return nil, fmt.Errorf("unexpected HTTP response: %d %s", resp.StatusCode, resp.Status)
	}
	return rpc.NewClient(conn), nil
}

func (c *RPCClient) Call(address string, method string, args interface{}, reply interface{}) error {
	client, err := dialHTTPTimeout("tcp", address, rpcDialTimeout)
	if err != nil {
		return err
	}
	defer client.Close()
	return client.Call(method, args, reply)
}
