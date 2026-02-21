package node

import (
	"net/rpc"
)

type RPCClient struct{}

func (c *RPCClient) Call(address string, method string, args interface{}, reply interface{}) error {
	client, err := rpc.DialHTTP("tcp", address)
	if err != nil {
		return err
	}
	defer client.Close()
	return client.Call(method, args, reply)
}
