package main

import (
	// Service needs to be imported here to be instantiated.

	_ "github.com/hy06ix/myprotocol/simulation"
	"github.com/hy06ix/onet/simul"
)

func main() {
	simul.Start()
}
