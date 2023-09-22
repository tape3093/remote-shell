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
	HOST      = "localhost"
	PORT      = "12345"
	TYPE      = "tcp"
	CERT_FILE = "./cert/client-cert.pem"
	KEY_FILE  = "./cert/client-key.pem"
	CA_FILE   = "./cert/ca-cert.pem"
)

type Client struct {
	connection net.Conn
}

func NewClient(address string) (*Client, error) {
	tlsConfig, err := LoadCertificates()
	if err != nil {
		log.Printf("Error loading certificates: %v", err)
		return nil, err
	}

	conn, err := tls.Dial(TYPE, address, tlsConfig)
	if err != nil {
		log.Printf("Error establishing client-server connection: %v", err)
		return nil, err
	}

	return &Client{
		connection: conn,
	}, nil
}

func LoadCertificates() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(CERT_FILE, KEY_FILE)
	if err != nil {
		log.Printf("Error while loading certificate and key: %v", err)
		return nil, err
	}

	authority, err := os.ReadFile(CA_FILE)
	if err != nil {
		log.Printf("Error loading CA certificate: %v", err)
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
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Connection to server closed: %v", r)
			os.Exit(1)
		}
	}()

	_, err := fmt.Fprintf(c.connection, command+"\n")
	if err != nil {
		if err != nil && strings.Contains(err.Error(), "broken pipe") {
			// Server closed the connection, trigger a panic.
			panic(err)
		}
		log.Printf("Error while sending command: %v", err)
	}
}

func (c *Client) ReceiveResponse() {
	reader := bufio.NewReader(c.connection)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			} else {
				log.Printf("Error while reading response: %v", err)
			}
			break
		}

		// Check for EOF marker
		if line == "EOF\n" {
			break
		}
		if line == "TERMINATE\n" {
			os.Exit(9)
			return
		}

		fmt.Print(line)
	}
}

func (c *Client) Close() {
	c.connection.Close()
}

func main() {
	client, err := NewClient(HOST + ":" + PORT)
	if err != nil {
		log.Fatalf("Error trying to create client: %v", err)
		os.Exit(1)
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
