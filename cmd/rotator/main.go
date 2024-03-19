package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/rodrikv/rotator/proxy"
	"github.com/rodrikv/rotator/rotator"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	pm := proxy.NewProxyManager()

	linksFile := flag.String("l", "proxies.txt", "path to the file containing the proxies")
	urlLink := flag.String("s", "", "subscription link to fetch from")
	port := flag.Int("p", 9000, "port to listen on")
	host := flag.String("h", "127.0.0.1", "host to listen on")

	flag.Parse()

	r := rotator.NewRotator(*port, *host, pm)

	go func() {
		for {
			ctx1, cancel1 := context.WithCancel(context.Background())
			pm.ClearProxies()

			var notTestedProxies []string
			var err error

			if *urlLink != "" {
				notTestedProxies, err = proxy.ProxyFromUrl(*urlLink)
			}

			if len(notTestedProxies) == 0 {
				notTestedProxies, err = proxy.ProxyFromFile(*linksFile)
			}

			if err != nil {
				log.Panic(err)
			}

			finished := make(chan bool)
			ports := make([]int, 0)
			go proxy.RunPingTest(ctx1, notTestedProxies, &ports, finished)
			<-finished

			log.Println("finished pinging")

			for _, port := range proxy.Ports {
				err := pm.AddProxy("127.0.0.1:" + fmt.Sprint(port))
				if err != nil {
					log.Println("error", err)
				}
			}

			time.Sleep(time.Minute * 1)
			proxy.Ports = make([]int, 0)
			cancel1()
			log.Print("redoing...")
		}
	}()

	r.Start(ctx)
}
