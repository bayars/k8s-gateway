# Examples

This directory contains example clients for testing the gateway.

## gRPC Client

### Build

```bash
go build -o grpc_client grpc_client.go
```

### Usage

```bash
# Execute SSH command
./grpc_client \
  -server localhost:50051 \
  -fqdn router1.myCustomer.safabayar.net \
  -username admin \
  -password mypassword \
  -command "show version" \
  -protocol ssh

# Execute Telnet command
./grpc_client \
  -server localhost:50051 \
  -fqdn switch1.myCustomer.safabayar.net \
  -username admin \
  -password mypassword \
  -command "show interfaces" \
  -protocol telnet

# Execute NETCONF RPC
./grpc_client \
  -server localhost:50051 \
  -fqdn router2.myCustomer.safabayar.net \
  -username admin \
  -password mypassword \
  -command "<get-config><source><running/></source></get-config>" \
  -protocol netconf
```

## SSH Bastion Access

### Connect to Gateway

```bash
# Connect using client SSH key
ssh -i ../config/client_key -p 2222 admin@localhost
```

### From Gateway Shell

Once connected to the gateway bastion, you can connect to devices:

```bash
# This will prompt for the device password
ssh router1.myCustomer.safabayar.net
```

## Using grpcurl

If you have `grpcurl` installed, you can test without building the client:

```bash
# Install grpcurl
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

# Execute command
grpcurl -plaintext -d '{
  "fqdn": "router1.myCustomer.safabayar.net",
  "username": "admin",
  "password": "mypassword",
  "command": "show version",
  "protocol": "ssh"
}' localhost:50051 gateway.Gateway/ExecuteCommand
```

## Testing Against Kubernetes Deployment

```bash
# Get the external IP
export GATEWAY_IP=$(kubectl get svc gateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
```

### gRPC

```bash
./grpc_client \
  -server $GATEWAY_IP:50051 \
  -fqdn srl1.safabayar.net \
  -username admin \
  -password NokiaSrl1! \
  -command "show version"
```

### SSH Bastion

```bash
ssh -i ../config/client_key admin@$GATEWAY_IP
```

### gNMI

```bash
gnmic -a $GATEWAY_IP:57400 --insecure --encoding json_ietf \
  -u admin -p NokiaSrl1! \
  --target srl1.safabayar.net \
  get --path /system/information
```
