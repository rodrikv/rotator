package main

import (
	"context"
	"fmt"
	"log"

	"github.com/rodrikv/rotator/proxy"
	"github.com/rodrikv/rotator/rotator"
)

func main() {
	ctx := context.Background()

	pm := proxy.NewProxyManager()

	notTestedProxies, err := proxy.ProxyFromFile("proxies.txt")

	if err != nil {
		log.Panic(err)
	}

	r := rotator.NewRotator(9000, "127.0.0.1", pm)

	finished := make(chan bool)

	ports := make([]int, 0)

	go proxy.RunPingTest(ctx, notTestedProxies, &ports, finished)

	// check if the test is finished
	<-finished

	log.Println("finished pinging")

	for _, port := range ports {
		err := pm.AddProxy("http://127.0.0.1:" + fmt.Sprint(port))
		if err != nil {
			log.Println("error", err)
		}
	}
	r.Start(ctx)
}
