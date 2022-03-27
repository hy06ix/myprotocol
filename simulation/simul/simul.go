package main

import (
	// Service needs to be imported here to be instantiated.

	"github.com/csanti/onet/simul"
	_ "github.com/hy06ix/myprotocol/simulation"
)

func main() {
	simul.Start()
}
