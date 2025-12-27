package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/safabayar/gateway/internal/config"
	grpcserver "github.com/safabayar/gateway/internal/grpc"
	"github.com/safabayar/gateway/internal/logger"
	sshbastion "github.com/safabayar/gateway/internal/ssh"
	pb "github.com/safabayar/gateway/proto"
	"google.golang.org/grpc"
)

var (
	configPath        = flag.String("config", "config/devices.yaml", "Path to device configuration file")
	logPath           = flag.String("log", "logs/gateway.log", "Path to log file")
	grpcPort          = flag.Int("grpc-port", 50051, "gRPC server port")
	sshPort           = flag.Int("ssh-port", 2222, "SSH bastion server port")
	hostKeyPath       = flag.String("host-key", "config/ssh_host_key", "Path to SSH host key")
	authorizedKeysPath = flag.String("authorized-keys", "config/authorized_keys", "Path to authorized keys file")
)

func main() {
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	if err := logger.InitLogger(*logPath, cfg.Settings.LogLevel); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	logger.Log.Info("Starting Multi-Protocol Gateway")
	logger.Log.Infof("Loaded configuration for %d devices", len(cfg.Devices))

	// Create channels for coordinating shutdown
	errChan := make(chan error, 2)
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	// Start gRPC server
	go func() {
		if err := startGRPCServer(cfg, *grpcPort); err != nil {
			errChan <- fmt.Errorf("gRPC server error: %w", err)
		}
	}()

	// Start SSH bastion server
	go func() {
		if err := startSSHBastion(cfg, *sshPort, *hostKeyPath, *authorizedKeysPath); err != nil {
			errChan <- fmt.Errorf("SSH bastion error: %w", err)
		}
	}()

	logger.Log.Info("Gateway started successfully")
	logger.Log.Infof("gRPC server listening on port %d", *grpcPort)
	logger.Log.Infof("SSH bastion listening on port %d", *sshPort)
	logger.Log.Info("Press Ctrl+C to stop")

	// Wait for shutdown signal or error
	select {
	case err := <-errChan:
		logger.Log.WithError(err).Error("Server error")
		os.Exit(1)
	case sig := <-shutdownChan:
		logger.Log.Infof("Received signal %v, shutting down gracefully", sig)
	}

	logger.Log.Info("Gateway stopped")
}

func startGRPCServer(cfg *config.Config, port int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", port, err)
	}

	grpcServer := grpc.NewServer()
	gatewayServer := grpcserver.NewServer(cfg)

	pb.RegisterGatewayServer(grpcServer, gatewayServer)

	logger.Log.Infof("Starting gRPC server on port %d", port)

	if err := grpcServer.Serve(listener); err != nil {
		return fmt.Errorf("failed to serve gRPC: %w", err)
	}

	return nil
}

func startSSHBastion(cfg *config.Config, port int, hostKeyPath, authorizedKeysPath string) error {
	bastion, err := sshbastion.NewBastionServer(cfg, hostKeyPath, authorizedKeysPath)
	if err != nil {
		return fmt.Errorf("failed to create SSH bastion: %w", err)
	}

	logger.Log.Infof("Starting SSH bastion server on port %d", port)

	if err := bastion.Start(fmt.Sprintf(":%d", port)); err != nil {
		return fmt.Errorf("failed to start SSH bastion: %w", err)
	}

	return nil
}
