package gnmi

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"strings"
	"time"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/safabayar/gateway/internal/config"
	"github.com/safabayar/gateway/internal/logger"
)

// Server implements gNMI proxy server
type Server struct {
	gnmipb.UnimplementedGNMIServer
	config *config.Config
}

// NewServer creates a new gNMI proxy server
func NewServer(cfg *config.Config) *Server {
	return &Server{
		config: cfg,
	}
}

// getTargetFromContext extracts target device from gRPC metadata or target field
func (s *Server) getTargetFromContext(ctx context.Context, prefix *gnmipb.Path) (string, string, string, error) {
	// Try to get target from metadata headers
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		// Check for custom target header (x-gnmi-target)
		if targets := md.Get("x-gnmi-target"); len(targets) > 0 {
			return s.parseTarget(targets[0])
		}
		// Check for username/password in metadata
	}

	// Try to get from prefix target
	if prefix != nil && prefix.Target != "" {
		return s.parseTarget(prefix.Target)
	}

	return "", "", "", fmt.Errorf("no target specified in metadata or prefix")
}

// parseTarget parses target string like "srl1.safabayar.net:admin:password" or "srl1.safabayar.net"
func (s *Server) parseTarget(target string) (string, string, string, error) {
	parts := strings.Split(target, ":")
	fqdn := parts[0]
	username := "admin"
	password := "NokiaSrl1!"

	if len(parts) >= 2 {
		username = parts[1]
	}
	if len(parts) >= 3 {
		password = parts[2]
	}

	return fqdn, username, password, nil
}

// getBackendClient creates a gNMI client connection to the backend device
func (s *Server) getBackendClient(_ context.Context, fqdn, username, password string) (gnmipb.GNMIClient, *grpc.ClientConn, error) {
	device, deviceName, err := s.config.GetDeviceByFQDN(fqdn)
	if err != nil {
		return nil, nil, fmt.Errorf("device not found: %w", err)
	}

	// gNMI typically uses port 57400 for SR Linux
	gnmiPort := 57400
	if device.GNMIPort > 0 {
		gnmiPort = device.GNMIPort
	}

	target := fmt.Sprintf("%s:%d", device.Hostname, gnmiPort)
	logger.Log.WithFields(map[string]interface{}{
		"device": deviceName,
		"target": target,
	}).Debug("Connecting to backend gNMI server")

	// Create gRPC connection with TLS (skip verify for lab)
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		grpc.WithPerRPCCredentials(&basicAuth{
			username: username,
			password: password,
		}),
	}

	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		// Try without TLS
		opts = []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithPerRPCCredentials(&basicAuth{
				username: username,
				password: password,
				insecure: true,
			}),
		}
		conn, err = grpc.NewClient(target, opts...)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to connect to %s: %w", target, err)
		}
	}

	client := gnmipb.NewGNMIClient(conn)
	return client, conn, nil
}

// basicAuth implements credentials.PerRPCCredentials
type basicAuth struct {
	username string
	password string
	insecure bool
}

func (b *basicAuth) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"username": b.username,
		"password": b.password,
	}, nil
}

func (b *basicAuth) RequireTransportSecurity() bool {
	return !b.insecure
}

// Capabilities returns the gNMI capabilities of the target
func (s *Server) Capabilities(ctx context.Context, req *gnmipb.CapabilityRequest) (*gnmipb.CapabilityResponse, error) {
	fqdn, username, password, err := s.getTargetFromContext(ctx, nil)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	logger.Log.WithField("target", fqdn).Info("gNMI Capabilities request")

	client, conn, err := s.getBackendClient(ctx, fqdn, username, password)
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return client.Capabilities(ctx, req)
}

// Get retrieves data from the target
func (s *Server) Get(ctx context.Context, req *gnmipb.GetRequest) (*gnmipb.GetResponse, error) {
	fqdn, username, password, err := s.getTargetFromContext(ctx, req.Prefix)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	logger.Log.WithFields(map[string]interface{}{
		"target": fqdn,
		"paths":  len(req.Path),
	}).Info("gNMI Get request")

	client, conn, err := s.getBackendClient(ctx, fqdn, username, password)
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return client.Get(ctx, req)
}

// Set modifies data on the target
func (s *Server) Set(ctx context.Context, req *gnmipb.SetRequest) (*gnmipb.SetResponse, error) {
	fqdn, username, password, err := s.getTargetFromContext(ctx, req.Prefix)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	logger.Log.WithFields(map[string]interface{}{
		"target":  fqdn,
		"updates": len(req.Update),
		"deletes": len(req.Delete),
	}).Info("gNMI Set request")

	client, conn, err := s.getBackendClient(ctx, fqdn, username, password)
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return client.Set(ctx, req)
}

// Subscribe creates a subscription stream to the target
func (s *Server) Subscribe(stream gnmipb.GNMI_SubscribeServer) error {
	// Receive first message to get target info
	req, err := stream.Recv()
	if err != nil {
		return status.Error(codes.InvalidArgument, "failed to receive subscription request")
	}

	var fqdn, username, password string
	if sub := req.GetSubscribe(); sub != nil && sub.Prefix != nil {
		fqdn, username, password, err = s.getTargetFromContext(stream.Context(), sub.Prefix)
	} else {
		fqdn, username, password, err = s.getTargetFromContext(stream.Context(), nil)
	}
	if err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}

	logger.Log.WithField("target", fqdn).Info("gNMI Subscribe request")

	client, conn, err := s.getBackendClient(stream.Context(), fqdn, username, password)
	if err != nil {
		return status.Error(codes.Unavailable, err.Error())
	}
	defer conn.Close()

	// Create subscription to backend
	backendStream, err := client.Subscribe(stream.Context())
	if err != nil {
		return status.Error(codes.Internal, fmt.Sprintf("failed to create backend subscription: %v", err))
	}

	// Send the initial request
	if err := backendStream.Send(req); err != nil {
		return status.Error(codes.Internal, fmt.Sprintf("failed to send to backend: %v", err))
	}

	// Bidirectional proxy
	errChan := make(chan error, 2)

	// Forward from client to backend
	go func() {
		for {
			req, err := stream.Recv()
			if err == io.EOF {
				_ = backendStream.CloseSend()
				return
			}
			if err != nil {
				errChan <- err
				return
			}
			if err := backendStream.Send(req); err != nil {
				errChan <- err
				return
			}
		}
	}()

	// Forward from backend to client
	go func() {
		for {
			resp, err := backendStream.Recv()
			if err == io.EOF {
				errChan <- nil
				return
			}
			if err != nil {
				errChan <- err
				return
			}
			if err := stream.Send(resp); err != nil {
				errChan <- err
				return
			}
		}
	}()

	return <-errChan
}
