package main

import (
	"github.com/ningxin18/plasma_cash/loom_test/src/hostile_operator"

	"github.com/loomnetwork/go-loom/plugin"
)

var Contract = hostile_operator.Contract

func main() {
	plugin.Serve(Contract)
}
