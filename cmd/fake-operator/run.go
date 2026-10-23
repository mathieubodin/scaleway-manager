package main

import (
	"io"
	"math/rand/v2"
	"os"
)

func run(getenv func(string) string, stdout io.Writer, sigs <-chan os.Signal, tokenPath, caPath string, clock Clock, rnd *rand.Rand) int {
	return 1
}
