package rotator

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/rodrikv/rotator/forward"
	"github.com/rodrikv/rotator/proxy"
)

type Rotator struct {
	proxyManager *proxy.ProxyManager
	port         int
	host         string
}

func NewRotator(port int, host string, pm *proxy.ProxyManager) *Rotator {
	return &Rotator{
		proxyManager: pm,
		port:         port,
		host:         host,
	}
}

func (r *Rotator) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Handle SIGINT to cancel the context
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGINT)
		<-sigCh
		log.Println("Received SIGINT. Shutting down...")
		cancel()
	}()

	addr, err := net.ResolveTCPAddr("tcp", r.host+":"+strconv.Itoa(r.port))
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("listening on %s\n", addr.String())
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}

	for {
		conn, err := listener.Accept()
		log.Println(err)
		if err != nil {
			select {
			case <-ctx.Done():
				log.Println("Shutting down listener.")
				return
			default:
				log.Println("Error accepting connection:", err)
			}
		}

		go r.handleConnection(ctx, conn, r.proxyManager)
	}
}

func (r *Rotator) handleConnection(ctx context.Context, conn net.Conn, pm *proxy.ProxyManager) {
	log.Println("Handling connection")
	defer conn.Close()

	fwd, err := forward.New(conn)
	defer fwd.Close()
	if err != nil {
		log.Printf("%v", err)
		return
	}

	fwd.OnSelectRemote(func(req *http.Request) (forward.Remote, error) {
		return pm, nil
	})

	err = fwd.Forward()
	if err != nil {
		log.Print("Error:", err)
	}
}
