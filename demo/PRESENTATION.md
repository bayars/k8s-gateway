# Multi-Protocol Gateway - Presentation Demo

## Overview

A unified gateway that provides secure, centralized access to network devices through multiple protocols:

| Protocol | Port | Use Case |
|----------|------|----------|
| **gRPC** | 50051 | Command execution (SSH/Telnet/NETCONF) |
| **gNMI** | 57400 | Model-driven telemetry & configuration |
| **SSH Bastion** | 2222 | Interactive shell access |

```
                    ┌─────────────────────────────────────┐
                    │       Multi-Protocol Gateway        │
                    │                                     │
   gnmic/grpcurl ──►│  gNMI Proxy   │  gRPC Server       │
        ssh      ──►│  (57400)      │  (50051)           │
                    │               │                     │
                    │       SSH Bastion (2222)           │
                    └──────────────┬──────────────────────┘
                                   │
                    ┌──────────────┴──────────────┐
                    │                             │
               ┌────▼────┐                  ┌─────▼────┐
               │  srl1   │◄────e1-1────────►│   srl2   │
               │SR Linux │  10.1.1.0/24     │ SR Linux │
               └─────────┘                  └──────────┘
```

---

## Demo Setup

### Prerequisites

```bash
# Install gnmic (gNMI client)
curl -sL https://get-gnmic.openconfig.net | bash

# Install grpcurl (gRPC client)
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

# Ensure kubectl is configured
kubectl cluster-info
```

### Deploy Demo Environment

```bash
cd demo
./deploy-demo.sh
```

Or manually:

```bash
# 1. Deploy SR Linux topology
kubectl apply -f clabernetes-topology-containerlab.yaml

# 2. Wait for pods
kubectl wait --for=condition=ready pod -l topology=srlinux-demo --timeout=300s

# 3. Deploy gateway config
kubectl apply -f gateway-config-demo.yaml

# 4. Deploy gateway (from k8s directory)
kubectl apply -f ../k8s/deployment.yaml
```

### Get Gateway IP

```bash
# Get the LoadBalancer external IP
export GATEWAY_IP=$(kubectl get svc gateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
echo "Gateway IP: $GATEWAY_IP"

# Services available:
# - gRPC:  $GATEWAY_IP:50051
# - gNMI:  $GATEWAY_IP:57400
# - SSH:   $GATEWAY_IP:22
```

---

## Demo 1: gNMI with gnmic

> **Note**: SR Linux requires `--encoding json_ietf` for gNMI requests.

### 1.1 Get System Information

```bash
gnmic -a $GATEWAY_IP:57400 \
  --insecure \
  --encoding json_ietf \
  -u admin -p NokiaSrl1! \
  --target srl1.safabayar.net \
  get \
  --path /system/information
```

Expected output:
```json
[
  {
    "source": "$GATEWAY_IP:57400",
    "timestamp": 1767039160924527109,
    "updates": [
      {
        "Path": "srl_nokia-system:system/srl_nokia-system-info:information",
        "values": {
          "srl_nokia-system:system/srl_nokia-system-info:information": {
            "current-datetime": "2025-12-29T20:12:39.367Z",
            "description": "SRLinux-v25.10.1 7220 IXR-D3",
            "last-booted": "2025-12-29T19:16:01.401Z",
            "version": "v25.10.1-399-g90c1dbe35ef"
          }
        }
      }
    ]
  }
]
```

### 1.2 Get Interface Configuration

```bash
# Get all interfaces
gnmic -a $GATEWAY_IP:57400 \
  --insecure \
  --encoding json_ietf \
  -u admin -p NokiaSrl1! \
  --target srl1.safabayar.net \
  get \
  --path /interface

# Get specific interface
gnmic -a $GATEWAY_IP:57400 \
  --insecure \
  --encoding json_ietf \
  -u admin -p NokiaSrl1! \
  --target srl1.safabayar.net \
  get \
  --path /interface[name=ethernet-1/1]
```

### 1.3 Get Device Capabilities

```bash
gnmic -a $GATEWAY_IP:57400 \
  --insecure \
  -u admin -p NokiaSrl1! \
  --target srl1.safabayar.net \
  capabilities
```

### 1.4 Configure Interface via gNMI Set

```bash
# Set interface description
gnmic -a $GATEWAY_IP:57400 \
  --insecure \
  --encoding json_ietf \
  -u admin -p NokiaSrl1! \
  --target srl1.safabayar.net \
  set \
  --update-path /interface[name=ethernet-1/1]/description \
  --update-value '"Configured via Gateway gNMI"'
```

### 1.5 Subscribe to Telemetry Stream

```bash
# Subscribe to interface counters (STREAM mode)
gnmic -a $GATEWAY_IP:57400 \
  --insecure \
  --encoding json_ietf \
  -u admin -p NokiaSrl1! \
  --target srl1.safabayar.net \
  subscribe \
  --path /interface[name=ethernet-1/1]/statistics \
  --mode stream \
  --stream-mode sample \
  --sample-interval 5s

# Subscribe to interface state changes (ON_CHANGE mode)
gnmic -a $GATEWAY_IP:57400 \
  --insecure \
  --encoding json_ietf \
  -u admin -p NokiaSrl1! \
  --target srl1.safabayar.net \
  subscribe \
  --path /interface[name=ethernet-1/1]/admin-state \
  --mode stream \
  --stream-mode on-change
```

### 1.6 Multi-Device Query

```bash
# Query both devices simultaneously
for device in srl1 srl2; do
  echo "=== ${device} ==="
  gnmic -a $GATEWAY_IP:57400 \
    --insecure \
    --encoding json_ietf \
    -u admin -p NokiaSrl1! \
    --target ${device}.safabayar.net \
    get \
    --path /system/name/host-name
done
```

---

## Demo 2: gRPC Command Execution

### 2.1 Execute SSH Command via gRPC

```bash
grpcurl -plaintext \
  -d '{
    "fqdn": "srl1.safabayar.net",
    "username": "admin",
    "password": "NokiaSrl1!",
    "command": "show version",
    "protocol": "ssh"
  }' \
  $GATEWAY_IP:50051 \
  gateway.Gateway/ExecuteCommand
```

Expected output:
```json
{
  "output": "Hostname             : srl1\nChassis Type         : 7220 IXR-D3\nPart Number          : Sim Part No.\nSerial Number        : Sim Serial No.\nSystem HW MAC Address: 1A:89:00:FF:00:00\nOS                   : SR Linux\nSoftware Version     : v25.10.1\nBuild Number         : 399-g90c1dbe35ef\nArchitecture         : x86_64\nLast Booted          : 2025-12-29T19:16:01.398Z\nTotal Memory         : 4005452 kB\nFree Memory          : 379468 kB\n"
}
```

### 2.2 Show Interface Status

```bash
grpcurl -plaintext \
  -d '{
    "fqdn": "srl1.safabayar.net",
    "username": "admin",
    "password": "NokiaSrl1!",
    "command": "show interface brief",
    "protocol": "ssh"
  }' \
  $GATEWAY_IP:50051 \
  gateway.Gateway/ExecuteCommand
```

### 2.3 Show Network Instance

```bash
grpcurl -plaintext \
  -d '{
    "fqdn": "srl1.safabayar.net",
    "username": "admin",
    "password": "NokiaSrl1!",
    "command": "show network-instance summary",
    "protocol": "ssh"
  }' \
  $GATEWAY_IP:50051 \
  gateway.Gateway/ExecuteCommand
```

### 2.4 Configure Device via SSH

```bash
grpcurl -plaintext \
  -d '{
    "fqdn": "srl1.safabayar.net",
    "username": "admin",
    "password": "NokiaSrl1!",
    "command": "enter candidate\nset / interface ethernet-1/2 description \"Configured via gRPC Gateway\"\ncommit now",
    "protocol": "ssh"
  }' \
  $GATEWAY_IP:50051 \
  gateway.Gateway/ExecuteCommand
```

---

## Demo 3: NETCONF via gRPC

### 3.1 Get Running Configuration

```bash
grpcurl -plaintext \
  -d '{
    "fqdn": "srl1.safabayar.net",
    "username": "admin",
    "password": "NokiaSrl1!",
    "command": "<get-config><source><running/></source></get-config>",
    "protocol": "netconf"
  }' \
  $GATEWAY_IP:50051 \
  gateway.Gateway/ExecuteCommand
```

### 3.2 Get Interface Configuration via NETCONF

```bash
grpcurl -plaintext \
  -d '{
    "fqdn": "srl1.safabayar.net",
    "username": "admin",
    "password": "NokiaSrl1!",
    "command": "<get><filter type=\"subtree\"><interface xmlns=\"urn:srl_nokia/interfaces\"/></filter></get>",
    "protocol": "netconf"
  }' \
  $GATEWAY_IP:50051 \
  gateway.Gateway/ExecuteCommand
```

---

## Demo 4: SSH Bastion Mode

### 4.1 Connect via SSH Bastion

```bash
# Connect to gateway and then to SR Linux
ssh -i demo/ssh-keys/private/client_key \
    -o StrictHostKeyChecking=no \
    admin@$GATEWAY_IP \
    "ssh srl1.safabayar.net"
```

### 4.2 SSH ProxyJump Configuration

Add to `~/.ssh/config`:

```ssh-config
Host gateway
    HostName <GATEWAY_IP>
    Port 22
    User admin
    IdentityFile ~/poc/gateway/demo/ssh-keys/private/client_key
    StrictHostKeyChecking no

Host srl*.safabayar.net
    ProxyJump gateway
    User admin
```

Then connect directly:

```bash
ssh srl1.safabayar.net
# Password: NokiaSrl1!
```

---

## Demo 5: End-to-End Network Verification

### 5.1 Verify Connectivity Between Nodes

```bash
# On srl1 - ping srl2
grpcurl -plaintext \
  -d '{
    "fqdn": "srl1.safabayar.net",
    "username": "admin",
    "password": "NokiaSrl1!",
    "command": "ping 10.1.1.2 count 3",
    "protocol": "ssh"
  }' \
  $GATEWAY_IP:50051 \
  gateway.Gateway/ExecuteCommand
```

### 5.2 Check ARP Table

```bash
grpcurl -plaintext \
  -d '{
    "fqdn": "srl1.safabayar.net",
    "username": "admin",
    "password": "NokiaSrl1!",
    "command": "show arpnd arp-entries",
    "protocol": "ssh"
  }' \
  $GATEWAY_IP:50051 \
  gateway.Gateway/ExecuteCommand
```

### 5.3 Compare Configurations via gNMI

```bash
# Get interface config from both devices
echo "=== srl1 interface ==="
gnmic -a $GATEWAY_IP:57400 --insecure --encoding json_ietf \
  -u admin -p NokiaSrl1! \
  --target srl1.safabayar.net \
  get --path /interface[name=ethernet-1/1]/subinterface[index=0]/ipv4/address

echo "=== srl2 interface ==="
gnmic -a $GATEWAY_IP:57400 --insecure --encoding json_ietf \
  -u admin -p NokiaSrl1! \
  --target srl2.safabayar.net \
  get --path /interface[name=ethernet-1/1]/subinterface[index=0]/ipv4/address
```

---

## Architecture Deep Dive

### FQDN-Based Device Routing

The gateway extracts device name from FQDN subdomain:

```
srl1.customer.safabayar.net
 │       │          │
 │       │          └── Domain suffix (configurable)
 │       └── Customer/tenant (optional)
 └── Device name → maps to devices.yaml entry
```

### Protocol Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                           Gateway                                   │
│                                                                     │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────────────────┐ │
│  │   gNMI      │    │   gRPC      │    │    SSH Bastion         │ │
│  │   Proxy     │    │   Server    │    │                         │ │
│  │  (57400)    │    │  (50051)    │    │       (2222)            │ │
│  └──────┬──────┘    └──────┬──────┘    └───────────┬─────────────┘ │
│         │                  │                       │               │
│         │           ┌──────┴──────┐                │               │
│         │           │   Protocol  │                │               │
│         │           │   Selector  │                │               │
│         │           └──────┬──────┘                │               │
│         │      ┌───────────┼───────────┐           │               │
│         │      │           │           │           │               │
│    ┌────┴────┐ ┌───────┐ ┌───────┐ ┌───────┐ ┌────┴────┐          │
│    │  gNMI   │ │  SSH  │ │Telnet│ │NETCONF│ │   SSH    │          │
│    │ Client  │ │ Proxy │ │Proxy │ │ Proxy │ │  Client  │          │
│    └────┬────┘ └───┬───┘ └───┬──┘ └───┬───┘ └────┬────┘          │
│         │         │         │        │           │               │
└─────────┼─────────┼─────────┼────────┼───────────┼───────────────┘
          │         │         │        │           │
          ▼         ▼         ▼        ▼           ▼
      ┌───────────────────────────────────────────────┐
      │            Backend Network Devices            │
      │   (SR Linux: SSH/22, NETCONF/830, gNMI/57400) │
      └───────────────────────────────────────────────┘
```

### Authentication Model

| Path | Client → Gateway | Gateway → Device |
|------|------------------|------------------|
| **gNMI** | Username/Password in metadata | Username/Password via gRPC creds |
| **gRPC** | In request body | SSH/Telnet password |
| **SSH** | Public key auth | Password from target lookup |

---

## Key Benefits

1. **Unified Access Point**: Single gateway for all protocols
2. **FQDN-Based Routing**: Dynamic device resolution
3. **Protocol Abstraction**: Execute commands via any protocol through gRPC
4. **gNMI Proxy**: Native gnmic support for SR Linux
5. **SSH Bastion**: Secure jump host with public key auth
6. **Kubernetes Native**: Designed for K8s deployment

---

## Troubleshooting

### Check Gateway Logs

```bash
kubectl logs -f deployment/gateway
```

### Test Direct Connectivity

```bash
# From gateway pod
kubectl exec -it deployment/gateway -- nc -zv srl1.default.svc.cluster.local 57400
kubectl exec -it deployment/gateway -- nc -zv srl1.default.svc.cluster.local 22
```

### Verify SR Linux gRPC Server

```bash
kubectl exec -it deployment/srl1 -- sr_cli "show system grpc-server"
```

### Check Service Endpoints

```bash
kubectl get svc | grep -E "(gateway|srl)"
kubectl get pods -l app=gateway
```

---

## Cleanup

```bash
cd demo
./cleanup-demo.sh
```

Or manually:

```bash
kubectl delete topology srlinux-demo
kubectl delete configmap gateway-config gateway-authorized-keys
kubectl delete secret gateway-ssh-keys
kubectl delete deployment gateway
kubectl delete svc gateway
```
