package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/rodrikv/rotator/proxy"
	"github.com/rodrikv/rotator/rotator"
)

func main() {
	ctx := context.Background()

	pm := proxy.NewProxyManager()

	linksFile := flag.String("l", "proxies.txt", "path to the file containing the proxies")
	port := flag.Int("p", 9000, "port to listen on")
	host := flag.String("h", "127.0.0.1", "host to listen on")

	flag.Parse()

	notTestedProxies, err := proxy.ProxyFromFile(*linksFile)

	if err != nil {
		log.Panic(err)
	}

	r := rotator.NewRotator(*port, *host, pm)

	finished := make(chan bool)

	ports := make([]int, 0)

	go proxy.RunPingTest(ctx, notTestedProxies, &ports, finished)

	// check if the test is finished
	<-finished

	log.Println("finished pinging")

	fmt.Println(proxy.Ports)

	for _, port := range proxy.Ports {
		err := pm.AddProxy("127.0.0.1:" + fmt.Sprint(port))
		if err != nil {
			log.Println("error", err)
		}
	}
	r.Start(ctx)
}
