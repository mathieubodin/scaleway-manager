package main

import (
	"io"
	"math/rand/v2"
	"os"
)

// run is the start-up guard, the polling loop and /healthz. Story 3 stub: it
// does nothing and returns 0.
func run(getenv func(string) string, stdout io.Writer, sigs <-chan os.Signal, tokenPath, caPath string, clock Clock, rnd *rand.Rand) int {
	return 0
}
