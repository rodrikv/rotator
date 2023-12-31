package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Kawaii-Konnections-KK-Limited/Hayasashiken/foreignusage"
	"github.com/Kawaii-Konnections-KK-Limited/Hayasashiken/models"
	"github.com/Kawaii-Konnections-KK-Limited/Hayasashiken/run"

	"github.com/gin-gonic/gin"
)

var testurl = "http://cp.cloudflare.com/"
var timeout int32 = 1000
var baseBroadcast = "127.0.0.1"
var upperBoundPingLimit int32 = 1000
var ports []int

var (
	proxies   []*httputil.ReverseProxy
	current   int
	proxyLock sync.Mutex
)

type responseJson struct {
	Links []string `json:"links"`
}

func getProxyList() []string {
	f, err := os.Open("links.json")

	if err == nil {
		defer f.Close()
		var pairs responseJson
		err = json.NewDecoder(f).Decode(&pairs)
		if err != nil {
			panic(err)
		}
		return pairs.Links
	}

	linksUrl := os.Getenv("LINKS_URL")
	if linksUrl == "" {
		linksUrl = "http://209.38.244.174:8080/links"
	}

	auth := os.Getenv("LINKS_AUTH")
	if auth == "" {
		auth = "10"
	}

	header := http.Header{}
	header.Set("Authorization", auth)

	log.Println("Getting proxy list from /links")

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 100,
		},
	}

	req := http.Request{
		Header: header,
	}

	req.URL, _ = url.Parse(linksUrl)
	req.Method = "GET"

	resp, err := client.Do(&req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Println(resp.StatusCode)

	var pairs responseJson
	err = json.NewDecoder(resp.Body).Decode(&pairs)
	if err != nil {
		panic(err)
	}

	log.Println("Got", len(pairs.Links), "proxies")

	return pairs.Links
}

func main() {
	ctx := context.WithoutCancel(context.Background())

	proxies := getProxyList()[:1000]

	linksSimplified := []models.LinksSsimplified{}

	for i, proxy := range proxies {
		linksSimplified = append(linksSimplified, models.LinksSsimplified{
			ID:   i,
			Link: proxy,
		})
	}
	// a code that tests links in batches of 200 and appends to the list
	// if the test is successful

	var pairs []foreignusage.Pair

	for i := 0; i < len(linksSimplified); i += 200 {
		end := i + 200
		if end > len(linksSimplified) {
			end = len(linksSimplified)
		}
		sub := linksSimplified[i:end]
		pairs = append(pairs, foreignusage.GetTestResults(&sub, &timeout, &upperBoundPingLimit, &testurl, &ctx)...)

		log.Print(len(pairs))
	}

	go Run(pairs, ctx)

	time.Sleep(time.Second * 10)

	reverse_proxies := []*httputil.ReverseProxy{}
	for _, u := range ports {
		parsedURL, err := url.Parse("http://" + baseBroadcast + ":" + fmt.Sprint(u))
		if err != nil {
			panic(err)
		}
		reverse_proxies = append(reverse_proxies, httputil.NewSingleHostReverseProxy(parsedURL))
	}

	// Create a new Gin router
	router := gin.Default()

	// Middleware to forward requests in a round-robin fashion
	router.Use(func(c *gin.Context) {
		proxyLock.Lock()
		proxy := reverse_proxies[current]
		current = (current + 1) % len(proxies)
		log.Println("sending request with context ", c, " to ", proxy)
		proxyLock.Unlock()

		proxy.ServeHTTP(c.Writer, c.Request)
		c.Abort()
	})

	// Start the Gin server on port 8080
	port := 8082
	fmt.Printf("Server is listening on :%d...\n", port)
	if err := router.Run(fmt.Sprintf(":%d", port)); err != nil {
		panic(err)
	}
}

func Run(pairs []foreignusage.Pair, ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT)
	go func() {

		<-sigCh

		fmt.Println("Received sigint signal")

		cancel() // Cancel the context when SIGINT is received

	}()

	var counts int = 0
	for i, v := range pairs {
		link := v.Link
		port := i + 50000

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
				returned = true
			}

			if returned && len(ports) == 0 {
				fmt.Println("all tested nothing works")
				return
			}

		}

	}

}
func start(link *string, port int, ctx context.Context, counts *int) {
	kills := make(chan bool, 1)

	r, _ := run.SingByLinkProxy(link, &testurl, &port, &timeout, &baseBroadcast, ctx, &kills)

	if r < upperBoundPingLimit && r != 0 {
		*counts++
		fmt.Println(r)
		ports = append(ports, port)

	} else {
		kills <- true
		*counts++
	}

}
