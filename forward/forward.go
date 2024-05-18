package forward

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Remote interface {
	GetRemoteAddr() (*net.TCPAddr, error)
}

type User interface {
	Limit() (count, reset int64, allow bool)
	IsConnected() bool
}

type Forward struct {
	log log.Logger

	conn net.Conn
	user User

	request  *http.Request
	response *http.Response

	authHandler   OnAuthenticationHandlerFunc
	remoteHandler OnToHandlerFunc
	httpHandler   []OnHandlerFunc

	maxRetry int
	data     interface{}

	rate    int64
	reset   int64
	allowed bool

	remoteConn net.Conn
}

var unauthorizedMsg = []byte("Proxy Authentication Required")
var errorMsg = []byte("Proxy internal error")

type OnAuthenticationHandlerFunc func(req *http.Request, username string, password string) (User, error)
type OnToHandlerFunc func(req *http.Request) (Remote, error)
type OnHandlerFunc func(resp *http.Response, req *http.Request) error

func New(conn net.Conn) (*Forward, error) {
	fwd := Forward{
		log:      *log.Default(),
		conn:     conn,
		maxRetry: 20,
	}
	return &fwd, nil
}

func (fwd *Forward) SetData(data interface{}) {
	fwd.data = data
}

func (fwd *Forward) GetData() interface{} {
	return fwd.data
}

func (fwd *Forward) GetUser() User {
	return fwd.user
}

func (fwd *Forward) Close() {
	fwd.conn.Close()
}

func (fwd *Forward) On(cb OnHandlerFunc) {
	fwd.httpHandler = append(fwd.httpHandler, cb)
}

func (fwd *Forward) OnAuthentication(cb OnAuthenticationHandlerFunc) {
	fwd.authHandler = cb
}

func (fwd *Forward) OnSelectRemote(cb OnToHandlerFunc) {
	fwd.remoteHandler = cb
}

func (fwd *Forward) Forward(ctx context.Context) (err error) {
	return fwd.fastForward(ctx)
}

func (fwd *Forward) fastForward(ctx context.Context) (err error) {
	fwd.remoteConn, err = fwd.getRemoteConn(30 * time.Second)

	var wg sync.WaitGroup
	wg.Add(2)

	done := make(chan struct{}, 2)
	connClosed := make(chan net.Conn, 2)

	go func() {
		defer wg.Done()
		copyData(ctx, fwd.conn, fwd.remoteConn, done, connClosed)
	}()

	go func() {
		defer wg.Done()
		copyData(ctx, fwd.remoteConn, fwd.conn, done, connClosed)
	}()

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Both goroutines have finished
	case <-ctx.Done():
		// Context was canceled
	}

	// err := fwd.readRequest()
	// if err != nil {
	// 	if strings.HasPrefix(err.Error(), "malformed HTTP") {
	// 		return fwd.forwardTunnel()
	// 	}
	// 	fwd.createErrorResponse(500, []byte("Failed to read sent request."))
	// 	return err
	// }

	// if fwd.user != nil && fwd.user.IsConnected() {
	// 	fwd.rate, fwd.reset, fwd.allowed = fwd.user.Limit()
	// 	if !fwd.allowed {
	// 		fwd.createErrorResponse(400, []byte("User rate limits exceeded."))
	// 		return errors.New("User rate limits exceeded.")
	// 	}
	// }

	// err = nil
	// for fwd.maxRetry > 0 {
	// 	err = fwd.forward()
	// 	log.Println("forwarding")
	// 	if err == nil {
	// 		break
	// 	}
	// 	fwd.maxRetry--
	// }

	// if err != nil {
	// 	fmt.Println("HERE")
	// 	fwd.createErrorResponse(500, []byte(err.Error()))
	// 	return err
	// }

	// defer func() {
	// 	if fwd.remoteConn != nil {
	// 		fwd.remoteConn.Close()
	// 	}
	// }()

	// // Send request and response to callbacks
	// // The user can manage request and response
	// // before they are sent back.
	// for _, cb := range fwd.httpHandler {
	// 	err = cb(fwd.response, fwd.request)
	// 	if err != nil {
	// 		fmt.Println("On Response Handler err")
	// 		return err
	// 	}
	// }

	// // Send back remote proxy host response to initial
	// // client.
	// err = fwd.response.Write(fwd.conn)
	// if err != nil {
	// 	fmt.Println("Error Writing")
	// }
	// if fwd.request.Method == http.MethodConnect {
	// 	log.Println(fwd.remoteConn, fwd.conn)
	// 	return fwd.forwardTunnel()
	// }
	// return err

	// log.Print("client connection: ", fwd.conn.RemoteAddr(), " remote connection: ", fwd.remoteConn.RemoteAddr())

	return nil
}

func (fwd *Forward) getRemoteConn(timeout time.Duration) (net.Conn, error) {
	if fwd.remoteHandler == nil {
		return nil, errors.New("No callback for fwd.OnSelectRemote() found. Can't perform request.")
	}

	remote, err := fwd.remoteHandler(fwd.request)
	if err != nil {
		return nil, err
	}

	remote_addr, err := remote.GetRemoteAddr()
	if err != nil {
		return nil, err
	}

	fwd.log.Printf("Trying with remote %v", remote_addr.String())
	conn, err := net.DialTimeout("tcp", remote_addr.String(), timeout)
	if err != nil {
		return nil, err
	}
	conn.SetDeadline(time.Now().Add(timeout))

	return conn, nil
}

func (fwd *Forward) forward() error {
	timeout_delta := 30 * time.Second
	var err error
	fwd.remoteConn, err = fwd.getRemoteConn(timeout_delta)
	if err != nil {
		fmt.Println("getRemoteConn?")
		return err
	}

	// Forward request to remote proxy host.
	err = fwd.request.WriteProxy(fwd.remoteConn)
	if err != nil {
		fmt.Println("WriteProxy?", err)
		return err
	}

	// Read response from remote proxy host.
	// Here we NEED to check status code and other stuff
	// to get clean request and be able to serve only
	// valid content.
	// Status code to check :
	//   301 -> redirection
	//   4/5xx -> Check for error and retry
	err = fwd.readResponse(fwd.remoteConn)
	if err != nil {
		fmt.Println("ReadResponse?", err)
		return err
	}
	return nil
}

func (fwd *Forward) forwardTunnel() (err error) {
	// Handle the CONNECT request for HTTPS
	// Establish a tunnel between the client and the destination server

	// fwd.remoteConn, err = fwd.getRemoteConn(10 * time.Second)
	// if err != nil {
	// 	log.Printf("Error connecting to destination server: %v", err)
	// 	return err
	// }

	err = fwd.request.WriteProxy(fwd.remoteConn)
	if err != nil {
		log.Println("WriteProxy?", err)
		return err
	}

	log.Println(fwd.remoteConn.RemoteAddr(), fwd.conn.RemoteAddr())

	// Respond to the client that the tunnel has been established
	// _, err = fwd.conn.Write([]byte("HTTP/1.0 200 Connection established\r\n\r\n"))
	// if err != nil {
	// 	return err
	// }

	var wg sync.WaitGroup
	wg.Add(2) // Wait for two goroutines to finish copying

	// Relay data from client to destination server
	go func() {
		defer wg.Done()
		_, err := io.Copy(fwd.remoteConn, fwd.conn)
		if err != nil {
			log.Printf("Error copying data to remote server: %v", err)
		}

		log.Printf("end of coping data from %s to %s", fwd.conn.RemoteAddr().String(), fwd.remoteConn.RemoteAddr().String())
	}()

	// Relay data from destination server to client
	go func() {
		defer wg.Done()
		_, err := io.Copy(fwd.conn, fwd.remoteConn)
		if err != nil {
			log.Printf("Error copying data to client: %v", err)
		}

		log.Printf("end of coping data from %s to %s", fwd.remoteConn.RemoteAddr().String(), fwd.conn.RemoteAddr().String())
	}()

	// Wait for both copying operations to finish
	wg.Wait()

	log.Printf("here")

	return err
}

func copyData(ctx context.Context, src, dst net.Conn, done chan<- struct{}, connClosed chan<- net.Conn) {
	defer func() {
		if src == nil {
			return
		}

		src.Close()
		connClosed <- src
	}()

	defer func() {
		if dst == nil {
			return
		}

		dst.Close()
		connClosed <- dst
	}()

	var mu sync.Mutex
	closed := false

	go func() {
		<-ctx.Done()
		mu.Lock()
		if !closed {
			closed = true
			src.Close()
			dst.Close()
		}
		mu.Unlock()
	}()

	if src == nil || dst == nil {
		return	
	}

	_, err := io.Copy(dst, src)
	mu.Lock()
	defer mu.Unlock()
	if err != nil && !closed {
		if err == io.ErrClosedPipe {
			// The other end of the connection has been closed,
			// so we can gracefully exit without logging an error.
			closed = true
			return
		}
		log.Printf("Error copying data: %v", err)
		// You could try to send the error back to the handleConnection function
		// or take other appropriate actions
	}

	done <- struct{}{}
}

func (fwd *Forward) readRequest() error {
	reader := bufio.NewReader(fwd.conn)

	req, err := http.ReadRequest(reader)
	if err != nil {
		return err
	}
	fwd.request = req

	dump, err := httputil.DumpRequest(fwd.request, false)
	if err == nil {
		fwd.log.Printf("Request :\n%v", string(dump))
	}

	err = fwd.filterRequest()
	if err != nil {
		return err
	}
	return nil
}

func (fwd *Forward) filterRequest() error {
	if fwd.request == nil {
		return errors.New("Can't filter forward request. Request is nil.")
	}

	// clean up necessary stuff
	fwd.request.Header.Del("Connection")
	fwd.request.Header.Del("Accept-Encoding")

	// check for headers specifics operations
	for k, _ := range fwd.request.Header {
		if strings.HasPrefix(k, "Proxy-") {
			// Handles the following headers and remove them
			// Proxy-Authorization: Basic dGVzdDp0ZXN0
			// Proxy-Connection: Keep-Alive

			switch k {
			case "Proxy-Authorization":
				// err := fwd.authenticate()
				// if err != nil {
				// 	fwd.createErrorResponse(407, unauthorizedMsg)
				// 	return err
				// }

			default:
			}

			fwd.request.Header.Del(k)
		}

		if strings.HasPrefix(k, "X-Proxifier") {
			switch k {
			// X-Proxifier-Https:
			// This header made the http initial request to be transformed
			// to an https request.
			case "X-Proxifier-Https":
				fwd.request.URL.Scheme = "https"
				r := strings.NewReplacer("http://", "https://")
				fwd.request.RequestURI = r.Replace(fwd.request.RequestURI)
			default:
			}

			fwd.request.Header.Del(k)
		}
	}

	// Check if we have a callback for authentication. if true, then we need to have
	// a valid user set.
	if fwd.authHandler != nil && fwd.user == nil {
		fwd.createErrorResponse(407, unauthorizedMsg)
		return errors.New("You need to send your authentication credentials")
	}

	return nil
}

func (fwd *Forward) readResponse(remote net.Conn) error {
	resp, err := http.ReadResponse(bufio.NewReader(remote), fwd.request)
	if err != nil {
		fmt.Println("ReadResponse with NewReader")
		return err
	}
	fwd.response = resp

	dump, err := httputil.DumpResponse(fwd.response, false)
	if err == nil {
		fwd.log.Printf("Response :\n%v", string(dump))
	}

	// fwd.remoteConn, err = fwd.filterResponse()
	// if err != nil {
	// 	fmt.Println("FilterResponse")
	// 	return err
	// }
	return nil
}

func (fwd *Forward) filterResponse() (net.Conn, error) {
	if fwd.response == nil {
		return nil, errors.New("Can't filter forwarded response. Response is nil.")
	}
	var remote net.Conn

	// In case of redirect, perform the redirect.
	if fwd.response.StatusCode == 301 {
		url, err := fwd.response.Location()
		if err != nil {
			return nil, err
		}
		fwd.request.URL = url
		fwd.request.RequestURI = url.String()

		fwd.remoteConn.Close()
		err = fwd.forward()

		if err != nil {
			return nil, err
		}
	}

	fwd.response.Header.Set("X-RateLimit-Limit", strconv.FormatInt(60, 10))
	fwd.response.Header.Set("X-RateLimit-Remaining", strconv.FormatInt((60-fwd.rate), 10))
	fwd.response.Header.Set("X-RateLimit-Reset", strconv.FormatInt(fwd.reset, 10))

	if fwd.response.StatusCode != 200 {
		return nil, errors.New("No 200 status code response")
	}
	return remote, nil
}

func (fwd *Forward) createErrorResponse(code int, reason []byte) {
	reason = append(reason, byte('\n'))
	fwd.response = &http.Response{
		StatusCode:    code,
		ProtoMajor:    1,
		ProtoMinor:    1,
		Request:       fwd.request,
		Body:          ioutil.NopCloser(bytes.NewBuffer(reason)),
		ContentLength: int64(len(reason)),
	}

	if code == 407 {
		// Automaticaly add a Proxy-Authenticate Header when the client need to
		// be logged.
		fwd.response.Header = http.Header{"Proxy-Authenticate": []string{"Basic realm="}}
	}

	fwd.response.Write(fwd.conn)
}
