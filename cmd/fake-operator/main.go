package main

import (
	"math/rand/v2"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func main() {
	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	rnd := rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 1))
	const dir = "/var/run/secrets/kubernetes.io/serviceaccount/"
	os.Exit(run(os.Getenv, os.Stdout, sigs, dir+"token", dir+"ca.crt", systemClock{}, rnd))
}
