package grpc

import (
	"context"
	"fmt"
	"io"

	"github.com/safabayar/gateway/internal/config"
	"github.com/safabayar/gateway/internal/logger"
	"github.com/safabayar/gateway/internal/proxy"
	pb "github.com/safabayar/gateway/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server implements the Gateway gRPC service
type Server struct {
	pb.UnimplementedGatewayServer
	config *config.Config
}

// NewServer creates a new gRPC server instance
func NewServer(cfg *config.Config) *Server {
	return &Server{
		config: cfg,
	}
}

// ExecuteCommand executes a single command on a device
func (s *Server) ExecuteCommand(ctx context.Context, req *pb.CommandRequest) (*pb.CommandResponse, error) {
	logger.Log.WithFields(map[string]interface{}{
		"fqdn":     req.Fqdn,
		"username": req.Username,
		"protocol": req.Protocol,
		"command":  req.Command,
	}).Info("Received command execution request")

	// Validate request
	if req.Fqdn == "" {
		return nil, status.Error(codes.InvalidArgument, "FQDN is required")
	}
	if req.Username == "" {
		return nil, status.Error(codes.InvalidArgument, "username is required")
	}
	if req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}
	if req.Command == "" {
		return nil, status.Error(codes.InvalidArgument, "command is required")
	}

	// Get device configuration
	device, deviceName, err := s.config.GetDeviceByFQDN(req.Fqdn)
	if err != nil {
		logger.Log.WithError(err).Error("Failed to get device config")
		return nil, status.Error(codes.NotFound, err.Error())
	}

	logger.Log.WithFields(map[string]interface{}{
		"device":   deviceName,
		"hostname": device.Hostname,
	}).Info("Routing to device")

	// Execute command based on protocol
	var output string
	var execErr error

	switch req.Protocol {
	case "ssh", "":
		output, execErr = proxy.ExecuteSSHCommand(
			device.Hostname,
			device.SSHPort,
			req.Username,
			req.Password,
			req.Command,
		)
	case "telnet":
		output, execErr = proxy.ExecuteTelnetCommand(
			device.Hostname,
			device.TelnetPort,
			req.Username,
			req.Password,
			req.Command,
		)
	case "netconf":
		output, execErr = proxy.ExecuteNetconfCommand(
			device.Hostname,
			device.NetconfPort,
			req.Username,
			req.Password,
			req.Command,
		)
	default:
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("unsupported protocol: %s", req.Protocol))
	}

	response := &pb.CommandResponse{
		Output: output,
	}

	if execErr != nil {
		response.Error = execErr.Error()
		response.ExitCode = 1
		logger.Log.WithError(execErr).Error("Command execution failed")
	} else {
		response.ExitCode = 0
		logger.Log.Info("Command executed successfully")
	}

	return response, nil
}

// StreamCommand handles streaming command execution for interactive sessions
func (s *Server) StreamCommand(stream pb.Gateway_StreamCommandServer) error {
	logger.Log.Info("Starting stream command session")

	var deviceName string
	var device *config.DeviceConfig
	var username, password string
	var protocol string

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			logger.Log.Info("Stream closed by client")
			return nil
		}
		if err != nil {
			logger.Log.WithError(err).Error("Error receiving stream")
			return err
		}

		// First message should contain connection details
		if device == nil {
			var err error
			device, deviceName, err = s.config.GetDeviceByFQDN(req.Fqdn)
			if err != nil {
				return status.Error(codes.NotFound, err.Error())
			}
			username = req.Username
			password = req.Password
			protocol = req.Protocol
			if protocol == "" {
				protocol = "ssh"
			}

			logger.Log.WithFields(map[string]interface{}{
				"device":   deviceName,
				"username": username,
				"protocol": protocol,
			}).Info("Stream session initialized")
		}

		// Execute command
		var output string
		var execErr error

		switch protocol {
		case "ssh":
			output, execErr = proxy.ExecuteSSHCommand(
				device.Hostname,
				device.SSHPort,
				username,
				password,
				req.Command,
			)
		case "telnet":
			output, execErr = proxy.ExecuteTelnetCommand(
				device.Hostname,
				device.TelnetPort,
				username,
				password,
				req.Command,
			)
		case "netconf":
			output, execErr = proxy.ExecuteNetconfCommand(
				device.Hostname,
				device.NetconfPort,
				username,
				password,
				req.Command,
			)
		}

		response := &pb.CommandResponse{
			Output: output,
		}

		if execErr != nil {
			response.Error = execErr.Error()
			response.ExitCode = 1
		} else {
			response.ExitCode = 0
		}

		if err := stream.Send(response); err != nil {
			logger.Log.WithError(err).Error("Error sending stream response")
			return err
		}
	}
}
