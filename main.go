package main

import (
	"bufio"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync/atomic"
)

var (
	u1, _   = url.Parse("http://localhost:2080")
	u2, _   = url.Parse("http://localhost:2080")
	u3, _   = url.Parse("http://localhost:2080")
	proxies = []*url.URL{
		u1,
		u2,
		u3,
	}
	counter int32
)

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
	// http.HandleFunc("/", handler)

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

	bf := bufio.NewReader(conn)
	req, err := http.ReadRequest(bf)
	if err != nil {
		log.Println("error", err)
		return
	}

	if req.Method == http.MethodConnect {
		handleConnectRequest(conn, req)
		return
	}
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

	err = fwd.Forward(req)

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
	return net.ResolveTCPAddr("tcp", "127.0.0.1:2080")
}

func getProxyURL() *url.URL {
	i := atomic.AddInt32(&counter, 1)
	return proxies[i%int32(len(proxies))]
}

func handleConnectRequest(conn net.Conn, req *http.Request) {
	// Handle the CONNECT request for HTTPS
	// Establish a tunnel between the client and the destination server

	host := req.Host
	log.Print(host)
	// if err != nil {
	// 	log.Printf("Error parsing CONNECT request host: %v", err)
	// 	return
	// }

	destConn, err := net.Dial("tcp", host)
	if err != nil {
		log.Printf("Error connecting to destination server: %v", err)
		return
	}
	defer destConn.Close()

	// Respond to the client that the tunnel has been established
	conn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))

	// Relay data between the client and the destination server
	go func() {
		io.Copy(destConn, conn)
	}()

	io.Copy(conn, destConn)
}
