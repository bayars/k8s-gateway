package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	pb "github.com/safabayar/gateway/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	serverAddr = flag.String("server", "localhost:50051", "Gateway server address")
	fqdn       = flag.String("fqdn", "router1.myCustomer.safabayar.net", "Target device FQDN")
	username   = flag.String("username", "admin", "Username for device authentication")
	password   = flag.String("password", "", "Password for device authentication")
	command    = flag.String("command", "show version", "Command to execute")
	protocol   = flag.String("protocol", "ssh", "Protocol to use (ssh, telnet, netconf)")
)

func main() {
	flag.Parse()

	if *password == "" {
		log.Fatal("Password is required. Use -password flag")
	}

	// Create gRPC connection
	conn, err := grpc.Dial(*serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewGatewayClient(conn)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Execute command
	fmt.Printf("Executing command on %s...\n", *fqdn)
	fmt.Printf("Protocol: %s\n", *protocol)
	fmt.Printf("Command: %s\n\n", *command)

	req := &pb.CommandRequest{
		Fqdn:     *fqdn,
		Username: *username,
		Password: *password,
		Command:  *command,
		Protocol: *protocol,
	}

	resp, err := client.ExecuteCommand(ctx, req)
	if err != nil {
		log.Fatalf("Error executing command: %v", err)
	}

	// Display results
	fmt.Println("=== Output ===")
	fmt.Println(resp.Output)

	if resp.Error != "" {
		fmt.Println("\n=== Error ===")
		fmt.Println(resp.Error)
	}

	fmt.Printf("\nExit Code: %d\n", resp.ExitCode)

	if resp.SessionId != "" {
		fmt.Printf("Session ID: %s\n", resp.SessionId)
	}
}
