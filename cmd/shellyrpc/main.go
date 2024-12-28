package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"github.com/hoegaarden/go-shellyrpc"
)

// Calls the specified RPC method on the Shelly device at the specified
// address, and dumps the response's result to stdout.
// Method names and parameters can be found, e.g. for the BLU TRV, at
// https://shelly-api-docs.shelly.cloud/docs-ble/Devices/trv#rpc-commands .
func main() {
	var addr string
	var method string
	var params string

	flag.StringVar(&addr, "addr", "f8:44:77:21:12:55", "Shelly device address")
	flag.StringVar(&method, "method", "Shelly.GetConfig", "RPC method to call")
	flag.StringVar(&params, "params", "null", "RPC method parameters as JSON blob")

	flag.Parse()

	rpcClient := &shellyrpc.Client{Address: addr}

	err := rpcClient.Setup()
	if err != nil {
		log.Printf("Failed to setup RPC client: %v", err)
		return
	}
	defer func() {
		if err := rpcClient.Teardown(); err != nil {
			log.Fatalf("Failed to teardown RPC client: %v", err)
		}
	}()

	reqParams := shellyrpc.Params{}
	err = json.Unmarshal([]byte(params), &reqParams)
	if err != nil {
		log.Printf("Failed to unmarshal params: %v", err)
		return
	}

	res, err := rpcClient.Call(method, reqParams)
	if err != nil {
		log.Printf("Failed to call %s with %q: %v", method, reqParams, err)
		return
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	err = enc.Encode(res)
	if err != nil {
		log.Printf("Failed to encode response: %v", err)
		return
	}
}
