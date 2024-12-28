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

	os.Exit(run(addr, method, params))
}

func run(addr, method, params string) (rc int) {
	rpcClient := &shellyrpc.Client{Address: addr}

	err := rpcClient.Setup()
	if err != nil {
		log.Printf("Failed to setup RPC client: %v", err)
		return 10
	}
	defer func() {
		if err := rpcClient.Teardown(); err != nil {
			log.Printf("Failed to teardown RPC client: %v", err)
			rc = 90
		}
	}()

	reqParams := shellyrpc.Params{}
	err = json.Unmarshal([]byte(params), &reqParams)
	if err != nil {
		log.Printf("Failed to unmarshal params: %v", err)
		return 20
	}

	res, err := rpcClient.Call(method, reqParams)
	if err != nil {
		switch e := err.(type) {
		case shellyrpc.RPCError:
			res = shellyrpc.Result{"RPCError": e}
			rc = 30
		default:
			log.Printf(`Failed to call "%s" with "%v": %v`, method, reqParams, e)
			return 40
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	err = enc.Encode(res)
	if err != nil {
		log.Printf("Failed to encode response: %v", err)
		return 50
	}

	return rc
}
