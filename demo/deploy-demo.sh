#!/bin/bash
set -e

echo "=================================="
echo "Gateway Demo Deployment Script"
echo "=================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Namespace
NAMESPACE=${NAMESPACE:-default}

echo -e "${YELLOW}Step 1: Deploying SR Linux topology with clabernetes${NC}"
echo "Applying clabernetes topology..."
kubectl apply -f clabernetes-topology-containerlab.yaml

echo ""
echo -e "${YELLOW}Waiting for SR Linux pods to be ready...${NC}"
echo "This may take 2-3 minutes for SR Linux to fully boot..."

# Wait for pods to be running
for i in {1..60}; do
    READY=$(kubectl get pods -l topology=srlinux-demo -o jsonpath='{.items[*].status.phase}' 2>/dev/null | grep -o "Running" | wc -l)
    if [ "$READY" -eq 2 ]; then
        echo -e "${GREEN}✓ SR Linux pods are running!${NC}"
        break
    fi
    echo -n "."
    sleep 5
done

echo ""
kubectl get pods -l topology=srlinux-demo

echo ""
echo -e "${YELLOW}Step 2: Getting ClusterIP service names${NC}"
echo "Discovered services:"
kubectl get svc -l topology=srlinux-demo

# Get actual service names
SRL1_SVC=$(kubectl get svc -l topology=srlinux-demo,node=srl1 -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "srl1-srlinux-demo")
SRL2_SVC=$(kubectl get svc -l topology=srlinux-demo,node=srl2 -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "srl2-srlinux-demo")

echo ""
echo "SRL1 Service: ${SRL1_SVC}"
echo "SRL2 Service: ${SRL2_SVC}"

echo ""
echo -e "${YELLOW}Step 3: Updating gateway configuration with actual service names${NC}"

# Create updated ConfigMap with actual service names
cat > /tmp/gateway-config-updated.yaml <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: gateway-config
  namespace: ${NAMESPACE}
data:
  devices.yaml: |
    devices:
      srl1:
        hostname: "${SRL1_SVC}.${NAMESPACE}.svc.cluster.local"
        ssh_port: 22
        telnet_port: 23
        netconf_port: 830
        description: "SR Linux Node 1"
        location: "Demo Lab"

      srl2:
        hostname: "${SRL2_SVC}.${NAMESPACE}.svc.cluster.local"
        ssh_port: 22
        telnet_port: 23
        netconf_port: 830
        description: "SR Linux Node 2"
        location: "Demo Lab"

    settings:
      domain_suffix: "safabayar.net"
      default_timeout: 30
      max_sessions: 100
      log_level: "info"
EOF

kubectl apply -f /tmp/gateway-config-updated.yaml

echo ""
echo -e "${YELLOW}Step 4: Generating SSH keys for gateway${NC}"

# Generate SSH keys if they don't exist
if [ ! -f /tmp/ssh_host_key ]; then
    ssh-keygen -t ed25519 -f /tmp/ssh_host_key -N "" -C "gateway-host-key" > /dev/null 2>&1
    echo -e "${GREEN}✓ Generated SSH host key${NC}"
else
    echo "SSH host key already exists"
fi

if [ ! -f /tmp/client_key ]; then
    ssh-keygen -t ed25519 -f /tmp/client_key -N "" -C "gateway-client-key" > /dev/null 2>&1
    cp /tmp/client_key.pub /tmp/authorized_keys
    echo -e "${GREEN}✓ Generated client SSH key${NC}"
else
    echo "Client SSH key already exists"
fi

# Create Kubernetes secret for SSH keys
kubectl create secret generic gateway-ssh-keys \
    --from-file=ssh_host_key=/tmp/ssh_host_key \
    --from-file=authorized_keys=/tmp/authorized_keys \
    --dry-run=client -o yaml | kubectl apply -f -

echo -e "${GREEN}✓ SSH keys secret created${NC}"

echo ""
echo -e "${YELLOW}Step 5: Deploying gateway${NC}"
kubectl apply -f ../k8s/deployment.yaml

echo ""
echo -e "${YELLOW}Waiting for gateway deployment to be ready...${NC}"
kubectl wait --for=condition=available --timeout=120s deployment/gateway

echo ""
echo -e "${GREEN}=================================="
echo "✓ Demo Deployment Complete!"
echo "==================================${NC}"
echo ""

echo "Services:"
kubectl get svc | grep -E "gateway|srlinux-demo|NAME"

echo ""
echo "Pods:"
kubectl get pods | grep -E "gateway|srlinux-demo|NAME"

echo ""
echo -e "${GREEN}Next Steps:${NC}"
echo "1. Test SSH access to SR Linux via gateway:"
echo "   kubectl port-forward svc/gateway-ssh 2222:22"
echo "   ssh -i /tmp/client_key -p 2222 admin@localhost"
echo ""
echo "2. Test gRPC access:"
echo "   kubectl port-forward svc/gateway-grpc 50051:50051"
echo "   cd ../examples && go run grpc_client.go -fqdn srl1.customer.safabayar.net -username admin -password admin -command 'show version'"
echo ""
echo "3. Direct access to SR Linux (for testing):"
echo "   kubectl exec -it <srl1-pod-name> -- sr_cli"
echo ""
echo "4. View gateway logs:"
echo "   kubectl logs -f deployment/gateway"
echo ""

# Save connection info
cat > /tmp/demo-connection-info.txt <<EOF
Demo Connection Information
===========================

SR Linux Credentials:
  Username: admin
  Password: admin

Gateway SSH Key: /tmp/client_key

Service Names:
  SRL1: ${SRL1_SVC}.${NAMESPACE}.svc.cluster.local
  SRL2: ${SRL2_SVC}.${NAMESPACE}.svc.cluster.local

Port Forward Commands:
  Gateway SSH:  kubectl port-forward svc/gateway-ssh 2222:22
  Gateway gRPC: kubectl port-forward svc/gateway-grpc 50051:50051

  Direct SR Linux 1: kubectl port-forward svc/${SRL1_SVC} 2221:22
  Direct SR Linux 2: kubectl port-forward svc/${SRL2_SVC} 2222:22

Test Commands:
  SSH via Gateway:
    ssh -i /tmp/client_key -p 2222 admin@localhost
    > ssh srl1.customer.safabayar.net

  gRPC Command:
    cd examples
    go run grpc_client.go \\
      -server localhost:50051 \\
      -fqdn srl1.customer.safabayar.net \\
      -username admin \\
      -password admin \\
      -command "show version" \\
      -protocol ssh
EOF

echo -e "${YELLOW}Connection info saved to: /tmp/demo-connection-info.txt${NC}"
