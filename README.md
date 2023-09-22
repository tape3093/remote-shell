# remote-shell

Remote-shell server-client implementation using Go

## Description

The remote-shell application allows clients to connect to a server over a secure TLS connection and execute shell commands on the server. The server enforces client certificate verification to ensure secure communication. Dummy self-signed certificates are provided.

## Usage

### Starting the Server

To start the server, run the following command from main directory:
go run ./server

### Starting the Client

To start the client, run the following command from main directory:
go run ./client
