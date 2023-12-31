package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const (
	host           = "localhost"
	port           = "12345"
	connectionType = "tcp"
	certFile       = "./cert/server-cert.pem"
	keyFile        = "./cert/server-key.pem"
	caFile         = "./cert/ca-cert.pem"
	timeout        = 100 * time.Second // client timeouts after 100 seconds from connection start
)

type Server struct {
	wg         sync.WaitGroup
	listener   net.Listener
	shutdown   chan struct{}
	connection chan net.Conn
}

func NewServer(address string) (*Server, error) {
	tlsConfig, err := LoadCertificates()
	if err != nil {
		return nil, err
	}

	listener, err := tls.Listen(connectionType, address, tlsConfig)
	if err != nil {
		return nil, err
	}
	log.Println("Server is listening on:", address)

	return &Server{
		listener:   listener,
		shutdown:   make(chan struct{}),
		connection: make(chan net.Conn),
	}, nil
}

func LoadCertificates() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	authority, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(authority)

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    caCertPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}, nil
}

func (s *Server) AcceptConnections() {
	defer s.wg.Done()

	for {
		select {
		case <-s.shutdown:
			return
		default:
			conn, err := s.listener.Accept()
			if err != nil {
				continue
			}
			log.Println("New connection established from:", conn.RemoteAddr())
			s.connection <- conn
		}
	}
}

func (s *Server) handleConnections() {
	defer s.wg.Done()

	for {
		select {
		case <-s.shutdown:
			return
		case conn := <-s.connection:
			go s.handleConnection(conn)
		}
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		select {
		case <-s.shutdown:
			return
		case <-ctx.Done():
			// The client session timed out, close the connection and exit
			s.handleTimeout(conn)
			return
		default:
			// Continue handling commands
			command := scanner.Text()
			if command == "exit" {
				log.Printf("Connection from %s closed\n", conn.RemoteAddr())
				return
			}

			// Execute the command and send output to the client
			if err := s.executeCommand(conn, command); err != nil {
				log.Printf("Error executing command: %v\n", err)
				conn.Write([]byte("Error executing command:" + err.Error() + "\n"))
				conn.Write([]byte("EOF\n"))
			}

		}
	}

	scannerErr := scanner.Err()
	if scannerErr != nil {
		log.Println("Error while reading incoming message:", scannerErr)
	}
}

func (s *Server) handleTimeout(conn net.Conn) {
	conn.Write([]byte("Client session duration exceeded. Disconnecting...\n"))
	conn.Write([]byte("TERMINATE\n"))
	log.Printf("Client (%s) session duration exceeded\n", conn.RemoteAddr())
}

func (s *Server) executeCommand(conn net.Conn, command string) error {
	cmd := exec.Command("sh", "-c", command)

	// Set CPU time limit to 5 seconds
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	ulimitCmd := exec.Command("sh", "-c", "ulimit -t 5; exec \"$@\"", "--", command)
	ulimitCmd.SysProcAttr = cmd.SysProcAttr

	outputPipe, err := ulimitCmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := ulimitCmd.Start(); err != nil {
		return err
	}

	go func() {
		defer outputPipe.Close()
		_, err := io.Copy(conn, outputPipe)
		if err != nil {
			log.Printf("Error sending command output to client: %v", err)
		}
	}()

	if err := ulimitCmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr
		}
	}

	// Mark the end of the output
	conn.Write([]byte("\nEOF\n"))

	return nil
}

func (s *Server) Start() {
	s.wg.Add(2)
	go s.AcceptConnections()
	go s.handleConnections()
}

func (s *Server) Stop() {
	close(s.shutdown)
	s.listener.Close()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return
	case <-time.After(time.Second):
		log.Println("Connection timed out")
		return
	}
}

func main() {

	server, err := NewServer(host + ":" + port)
	if err != nil {
		log.Fatalf("Error creating server: %v", err)
	}

	server.Start()

	// Change the working directory to the root ("/")
	if err := os.Chdir("/"); err != nil {
		log.Fatalf("Error changing working directory: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Server will shut down")
	server.Stop()
	log.Println("Server is shut down")
}
