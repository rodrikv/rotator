package proxy

import (
	"errors"
	"net"
)

type ProxyManager struct {
	proxies []*net.TCPAddr
	current uint64
}

func NewProxyManager() *ProxyManager {
	return &ProxyManager{
		proxies: make([]*net.TCPAddr, 0),
		current: 0,
	}
}

func (p *ProxyManager) AddProxy(proxy string) error {
	if proxy == "" {
		return errors.New("empty proxy")
	}
	rip, err := net.ResolveTCPAddr("tcp", proxy)
	if err != nil {
		return err
	}
	p.proxies = append(p.proxies, rip)
	return nil
}

func (p *ProxyManager) AddProxies(proxies []string) {
	for _, proxy := range proxies {
		p.AddProxy(proxy)
	}
}

func (p *ProxyManager) GetRemoteAddr() (*net.TCPAddr, error) {
	if len(p.proxies) == 0 {
		return nil, errors.New("no proxy")
	}

	p.current = (p.current + 1) % uint64(len(p.proxies))
	return p.proxies[p.current], nil
}

func (p *ProxyManager) ClearProxies() {
	p.proxies = make([]*net.TCPAddr, 0)
}
