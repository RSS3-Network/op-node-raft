package main

import "github.com/RSS3-Network/op-node-raft-proxy/internal/cmd"

func main() {
	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}
