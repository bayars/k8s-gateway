# Gateway Demo with SR Linux

This demo deploys a complete environment with:
- 2 SR Linux nodes via clabernetes (using ClusterIP only)
- Multi-protocol gateway with gRPC and SSH bastion
- Automated deployment and testing

## Prerequisites

- Kubernetes cluster with clabernetes installed
- kubectl configured
- Docker images:
  - `ghcr.io/nokia/srlinux:latest`
  - Gateway image built and available

## Quick Deploy

```bash
cd demo
./deploy-demo.sh
```

This script will:
1. Deploy 2 SR Linux nodes with clabernetes
2. Configure ClusterIP services (no auto-expose)
3. Generate SSH keys for gateway
4. Deploy the gateway
5. Display connection information

## Manual Deployment

### Step 1: Deploy SR Linux Topology

```bash
kubectl apply -f clabernetes-topology-containerlab.yaml
```

Wait for pods to be ready:

```bash
kubectl get pods -l topology=srlinux-demo -w
```

### Step 2: Get Service Names

```bash
kubectl get svc -l topology=srlinux-demo
```

Note the service names (e.g., `srl1-srlinux-demo`, `srl2-srlinux-demo`).

### Step 3: Update Gateway Configuration

Edit `gateway-config-demo.yaml` and update the hostname fields with actual service names:

```yaml
devices:
  srl1:
    hostname: "<actual-srl1-service>.default.svc.cluster.local"
  srl2:
    hostname: "<actual-srl2-service>.default.svc.cluster.local"
```

Apply the configuration:

```bash
kubectl apply -f gateway-config-demo.yaml
```

### Step 4: Create SSH Keys Secret

```bash
# Generate keys
ssh-keygen -t ed25519 -f /tmp/ssh_host_key -N ""
ssh-keygen -t ed25519 -f /tmp/client_key -N ""
cp /tmp/client_key.pub /tmp/authorized_keys

# Create secret
kubectl create secret generic gateway-ssh-keys \
    --from-file=ssh_host_key=/tmp/ssh_host_key \
    --from-file=authorized_keys=/tmp/authorized_keys
```

### Step 5: Deploy Gateway

```bash
kubectl apply -f ../k8s/deployment.yaml
```

## Testing the Demo

### 1. Test Direct SR Linux Access (Verify Topology)

```bash
# Get pod names
kubectl get pods -l topology=srlinux-demo

# Access SR Linux CLI
kubectl exec -it <srl1-pod-name> -- sr_cli

# In SR Linux CLI:
show version
show interface
show network-instance
```

### 2. Test Gateway gRPC Access

```bash
# Port forward gateway gRPC
kubectl port-forward svc/gateway-grpc 50051:50051 &

# Run test client
cd ../examples
go run grpc_client.go \
  -server localhost:50051 \
  -fqdn srl1.customer.safabayar.net \
  -username admin \
  -password admin \
  -command "show version" \
  -protocol ssh
```

### 3. Test SSH Bastion Mode

```bash
# Port forward gateway SSH
kubectl port-forward svc/gateway-ssh 2222:22 &

# Connect to gateway
ssh -i /tmp/client_key -p 2222 admin@localhost

# From gateway shell, connect to SR Linux
ssh srl1.customer.safabayar.net
# Password: admin
```

### 4. Test NETCONF via Gateway

```bash
cd ../examples
go run grpc_client.go \
  -server localhost:50051 \
  -fqdn srl1.customer.safabayar.net \
  -username admin \
  -password admin \
  -command "<get><filter type=\"subtree\"><system xmlns=\"urn:nokia.com:sros:ns:yang:sr:conf\"/></filter></get>" \
  -protocol netconf
```

## Demo Scenarios

### Scenario 1: Execute Commands on Multiple Devices

```bash
# Check version on both SR Linux nodes
for device in srl1 srl2; do
  echo "=== $device ==="
  go run grpc_client.go \
    -server localhost:50051 \
    -fqdn ${device}.customer.safabayar.net \
    -username admin \
    -password admin \
    -command "show version"
done
```

### Scenario 2: Network Configuration via Gateway

```bash
# Configure interface on srl1
go run grpc_client.go \
  -server localhost:50051 \
  -fqdn srl1.customer.safabayar.net \
  -username admin \
  -password admin \
  -command "info interface ethernet-1/1"
```

### Scenario 3: SSH Jump Host Workflow

This demonstrates the gateway acting as a bastion/jump server:

```bash
# 1. Connect to gateway
ssh -i /tmp/client_key -p 2222 admin@localhost

# 2. From gateway, see available devices
# (The gateway welcome message will show available devices)

# 3. Connect to device through gateway
ssh srl1.customer.safabayar.net
# Enter password: admin

# 4. Now you're on the SR Linux device
show version
show interface brief
exit

# 5. Connect to second device
ssh srl2.customer.safabayar.net
# Enter password: admin
```

## Topology Details

### Network Topology

```
┌─────────────────────┐
│   Gateway Service   │
│  - gRPC (50051)     │
│  - SSH (2222)       │
└──────────┬──────────┘
           │
           │ ClusterIP Services
           │
      ┌────┴─────┐
      │          │
┌─────▼────┐ ┌──▼───────┐
│   srl1   │─│   srl2   │
│ SR Linux │ │ SR Linux │
└──────────┘ └──────────┘
     │              │
     └──────┬───────┘
         e1-1 link
    10.1.1.1/24 - 10.1.1.2/24
```

### Service Mapping

| Component | Service Type | Port | FQDN |
|-----------|-------------|------|------|
| Gateway gRPC | ClusterIP | 50051 | gateway-grpc.default.svc.cluster.local |
| Gateway SSH | LoadBalancer | 22 | gateway-ssh.default.svc.cluster.local |
| SRL1 | ClusterIP | 22, 830 | srl1-srlinux-demo.default.svc.cluster.local |
| SRL2 | ClusterIP | 22, 830 | srl2-srlinux-demo.default.svc.cluster.local |

## Troubleshooting

### SR Linux Pods Not Starting

```bash
# Check pod status
kubectl describe pod -l topology=srlinux-demo

# Check clabernetes controller logs
kubectl logs -n clabernetes-system deployment/clabernetes-controller

# Verify image pull
kubectl get events --sort-by='.lastTimestamp'
```

### Gateway Cannot Connect to SR Linux

```bash
# Test direct connectivity from gateway pod
kubectl exec -it deployment/gateway -- sh
ping srl1-srlinux-demo.default.svc.cluster.local
nc -zv srl1-srlinux-demo.default.svc.cluster.local 22

# Check gateway logs
kubectl logs -f deployment/gateway

# Verify service endpoints
kubectl get endpoints
```

### SSH Authentication Failed

```bash
# Verify SR Linux default credentials
kubectl exec -it <srl1-pod> -- sr_cli show system aaa

# Check if SSH is enabled on SR Linux
kubectl exec -it <srl1-pod> -- sr_cli show system management

# Test direct SSH to SR Linux
kubectl port-forward svc/srl1-srlinux-demo 2221:22
ssh -p 2221 admin@localhost
# Password: admin
```

### NETCONF Not Working

```bash
# Verify NETCONF is enabled
kubectl exec -it <srl1-pod> -- sr_cli show system management netconf

# Test NETCONF port
kubectl exec -it deployment/gateway -- sh
nc -zv srl1-srlinux-demo.default.svc.cluster.local 830
```

## Cleanup

```bash
# Delete everything
kubectl delete topology srlinux-demo
kubectl delete configmap gateway-config
kubectl delete secret gateway-ssh-keys
kubectl delete -f ../k8s/deployment.yaml

# Verify cleanup
kubectl get pods
kubectl get svc
```

## Advanced Usage

### Add More Devices

Edit `clabernetes-topology-containerlab.yaml` to add more SR Linux nodes:

```yaml
srl3:
  kind: nokia_srlinux
  image: ghcr.io/nokia/srlinux:latest
  type: ixrd3
  # ... config ...
```

Then update `gateway-config-demo.yaml` to include the new device.

### Custom SR Linux Configuration

Modify the `startup-config` section in the topology file to add custom configurations:

```yaml
startup-config: |
  enter candidate
  / system management ssh admin-state enable
  / interface ethernet-1/2 admin-state enable
  / interface ethernet-1/2 subinterface 0 ipv4 address 192.168.1.1/24
  commit stay
```

### Monitor Gateway Performance

```bash
# Watch gateway logs
kubectl logs -f deployment/gateway

# Monitor resource usage
kubectl top pod -l app=gateway

# Check service metrics
kubectl get --raw /apis/metrics.k8s.io/v1beta1/namespaces/default/pods/gateway-xxx
```

## Reference

- SR Linux Default Credentials: `admin/admin`
- Gateway SSH Key: `/tmp/client_key`
- Demo FQDN Pattern: `<device>.customer.safabayar.net`
- ClusterIP Services: No external exposure, accessed via gateway only
