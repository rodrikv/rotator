package proxy

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Kawaii-Konnections-KK-Limited/Hayasashiken/run"
)

var testurl = "https://www.google.com/"
var timeout int32 = 10000
var baseBroadcast = "127.0.0.1"
var upperBoundPingLimit int32 = 10000
var ports []int

func RunPingTest(ctx context.Context, pairs []string, ps *[]int, finished chan bool) {
	ctx, cancel := context.WithCancel(ctx)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT)
	go func() {
		<-sigCh
		fmt.Println("Received sigint signal")
		cancel()
	}()

	var counts int = 0
	for i, v := range pairs {
		link := v
		port := i + 30000

		go start(&link, port, ctx, &counts)
	}
	returned := false
	for {
		select {
		case <-ctx.Done():
			fmt.Println("Context is done")
			return
		default:
			if !returned && len(pairs) == counts {
				fmt.Println(ports)
				finished <- true
				returned = true
			}
			if returned && len(ports) == 0 {
				finished <- true
				fmt.Println("all tested nothing works")
				return
			}
		}
	}
}

func start(link *string, port int, ctx context.Context, counts *int) {
	kills := make(chan bool, 1)
	r, err := run.SingByLinkProxy(link, &testurl, &port, &timeout, &baseBroadcast, ctx, &kills)
	if err != nil {
		log.Println(err)
	}
	if r < upperBoundPingLimit && r != 0 {
		*counts++
		fmt.Println(r)
		ports = append(ports, port)
	} else {
		kills <- true
		*counts++
	}
}
