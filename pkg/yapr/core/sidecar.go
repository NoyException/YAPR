package core

import (
	"bufio"
	"log"
	"net"
	"net/http"
	"noy/router/pkg/router/logger"
	"noy/router/pkg/router/store"
	"os/exec"
	"strconv"
)

type Sidecar struct {
	store    store.Store
	port     int
	listener net.Listener
	router   *Router
}

func NewSidecar(s store.Store, port int) *Sidecar {
	return &Sidecar{
		store: s,
		port:  port,
	}
}

func (s *Sidecar) handle(conn net.Conn) {
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			logger.Errorf("Error closing connection: %v", err)
		}
	}(conn)
	s.handleAsGRPC(conn)
}

func (s *Sidecar) handleAsGRPC(conn net.Conn) {
	// 尝试解析为 gRPC 请求

}

func (s *Sidecar) handleAsHttp(conn net.Conn) {
	// Read the request from the connection
	req, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil {
		logger.Errorf("Error reading request: %v", err)
		return
	}

	logger.Infof("Request received from %v", req.URL)

	// Get the original destination port
	originalPort := req.URL.Port()

	// Convert originalPort to int
	origPort, err := strconv.Atoi(originalPort)
	if err != nil {
		log.Println("Error converting port:", err)
		return
	}

	targetPort := origPort
	if origPort == 50050 {
		targetPort = 50051
	} else if origPort == 50051 {
		targetPort = 50050
	}
	// Find the target port from the routing map
	//targetPort, ok := s.router.Route(origPort)
	//if !ok {
	//	log.Println("No routing rule for port:", origPort)
	//	return
	//}

	// Forward the request to the target port
	target := "http://localhost:" + strconv.Itoa(targetPort)
	req.URL.Scheme = "http"
	req.URL.Host = target

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Errorf("Error forwarding request: %v", err)
		return
	}
	defer resp.Body.Close()

	// Write the response back to the original connection
	resp.Write(conn)
}

func (s *Sidecar) Start() error {
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(s.port))
	if err != nil {
		return err
	}
	s.listener = listener

	if err := exec.Command("iptables", "-t", "nat", "-A", "OUTPUT", "-p", "tcp", "--dport", "50050", "-j", "REDIRECT", "--to-port", strconv.Itoa(s.port)).Run(); err != nil {
		return err
	}
	if err := exec.Command("iptables", "-t", "nat", "-A", "PREROUTING", "-p", "tcp", "--dport", "50050", "-j", "REDIRECT", "--to-port", strconv.Itoa(s.port)).Run(); err != nil {
		return err
	}

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return err
		}
		go s.handle(conn)
	}
}

func (s *Sidecar) Stop() error {
	// Remove the iptables rules
	if err := exec.Command("iptables", "-t", "nat", "-D", "OUTPUT", "-p", "tcp", "--dport", "50050", "-j", "REDIRECT", "--to-port", strconv.Itoa(s.port)).Run(); err != nil {
		return err
	}
	if err := exec.Command("iptables", "-t", "nat", "-D", "PREROUTING", "-p", "tcp", "--dport", "50050", "-j", "REDIRECT", "--to-port", strconv.Itoa(s.port)).Run(); err != nil {
		return err
	}
	if err := s.listener.Close(); err != nil {
		return err
	}
	return nil
}
