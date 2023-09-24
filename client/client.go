package main

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
)

const (
	host           = "localhost"
	port           = "12345"
	connectionType = "tcp"
	certFile       = "./cert/client-cert.pem"
	keyFile        = "./cert/client-key.pem"
	caFile         = "./cert/ca-cert.pem"
)

type Client struct {
	connection net.Conn
}

func NewClient(address string) (*Client, error) {
	tlsConfig, err := LoadCertificates()
	if err != nil {
		return nil, err
	}

	conn, err := tls.Dial(connectionType, address, tlsConfig)
	if err != nil {
		return nil, err
	}

	return &Client{
		connection: conn,
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
		Certificates:       []tls.Certificate{cert},
		RootCAs:            caCertPool,
		InsecureSkipVerify: true, // Disable hostname verification for self-signed certificates
	}, nil
}

func (c *Client) SendCommand(command string) {
	_, err := fmt.Fprintf(c.connection, command+"\n")
	if err != nil {
		if err != nil && strings.Contains(err.Error(), "broken pipe") {
			// Server closed the connection, trigger a panic.
			log.Fatal("Connection to server closed")
		}
		log.Printf("Error while sending command: %v", err)
	}
}

func (c *Client) ReceiveResponse() {
	reader := bufio.NewReader(c.connection)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("Error while reading response: %v", err)
			}
			break
		}

		// Check for EOF marker
		if line == "EOF\n" {
			break
		} else if line == "TERMINATE\n" {
			os.Exit(9)
		}

		fmt.Print(line)
	}
}

func (c *Client) Close() {
	c.connection.Close()
}

func main() {
	client, err := NewClient(host + ":" + port)
	if err != nil {
		log.Fatalf("Error trying to create client: %v", err)
	}
	defer client.Close()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("Enter command (or 'exit' to quit): ")
		scanner.Scan()
		command := scanner.Text()

		client.SendCommand(command)

		if command == "exit" {
			fmt.Println("Exiting...")
			return
		}

		client.ReceiveResponse()
	}
}
