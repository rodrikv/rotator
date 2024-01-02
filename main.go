package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Kawaii-Konnections-KK-Limited/Hayasashiken/foreignusage"
	"github.com/Kawaii-Konnections-KK-Limited/Hayasashiken/models"
	"github.com/Kawaii-Konnections-KK-Limited/Hayasashiken/run"
)

var (
	counter int32
)

var testurl = "https://www.google.com/"
var timeout int32 = 10000
var baseBroadcast = "127.0.0.1"
var upperBoundPingLimit int32 = 10000
var ports []int

var (
	reverse_proxies []*url.URL
	current         int
	proxyLock       sync.Mutex
)

type responseJson struct {
	Links []string `json:"links"`
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

type connResponseWriter struct {
	conn net.Conn
}

func (c *connResponseWriter) Header() http.Header {
	return http.Header{}
}

func (c *connResponseWriter) Write(data []byte) (int, error) {
	return c.conn.Write(data)
}

func (c *connResponseWriter) WriteHeader(statusCode int) {
	// No-op, since we're not handling HTTP response headers here
}

func main() {
	ctx := context.WithoutCancel(context.Background())

	proxies := getProxyList()

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

	for i := 0; i < len(linksSimplified); i += 1000 {
		end := i + 1000
		if end > len(linksSimplified) {
			end = len(linksSimplified)
		}
		sub := linksSimplified[i:end]
		pairs = append(pairs, foreignusage.GetTestResults(&sub, &timeout, &upperBoundPingLimit, &testurl, &ctx)...)

		log.Print(len(pairs))
	}

	go Run(pairs, ctx)

	time.Sleep(time.Second * 10)
	for _, u := range ports {
		parsedURL, err := url.Parse("http://" + baseBroadcast + ":" + fmt.Sprint(u))
		if err != nil {
			panic(err)
		}
		reverse_proxies = append(reverse_proxies, parsedURL)
	}

	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:8000")
	if err != nil {
		log.Println("error", err)
	}

	log.Printf("listening on idk: ")
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		log.Println("error", err)
	}

	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			log.Println("error", err)
		}

		go handleRequest(conn)
	}
}

func handleRequest(conn net.Conn) {
	defer conn.Close()

	// bf := bufio.NewReader(conn)
	// req, err := http.ReadRequest(bf)
	// if err != nil {
	// 	log.Println("error", err)
	// 	handleConnectRequest(conn, req)
	// 	return
	// }

	fwd, err := New(conn)
	defer fwd.Close()
	if err != nil {
		log.Printf("%v", err)
		return
	}

	fwd.OnSelectRemote(func(req *http.Request) (Remote, error) {
		p := &Proxy{}
		return p, nil
	})

	err = fwd.Forward()

	if err != nil {
		log.Print("error: ", err)
	}

	// bf := bufio.NewReader(conn)
	// req, err := http.ReadRequest(bf)
	// if err != nil {
	// 	log.Println("error", err)
	// 	return
	// }

	// log.Printf("%#v", req)

	// proxyURL := getProxyURL()
	// proxy := &httputil.ReverseProxy{
	// 	Rewrite: func(r *httputil.ProxyRequest) {
	// 		log.Println(r.In.Host)
	// 		log.Println(r.Out.URL.Scheme)
	// 		log.Printf("%#v", r.In)
	// 		r.Out.Host = r.In.Host // if desired
	// 	},
	// 	Transport: &http.Transport{
	// 		Proxy: http.ProxyURL(proxyURL),
	// 		TLSClientConfig: &tls.Config{
	// 			InsecureSkipVerify: true, // Set this to true if you want to ignore certificate errors
	// 		},
	// 	},
	// }

	// // Serve the request via the proxy
	// wrappedWriter := &connResponseWriter{conn: conn}

	// // Serve the request via the proxy
	// proxy.ServeHTTP(wrappedWriter, req)
}

type Proxy struct {
}

func (p *Proxy) GetRemoteAddr() (*net.TCPAddr, error) {

	proxyUrl := getProxyURL()

	return net.ResolveTCPAddr("tcp", proxyUrl.Host)
}

func getProxyURL() *url.URL {
	i := atomic.AddInt32(&counter, 1)
	return reverse_proxies[i%int32(len(reverse_proxies))]
}
