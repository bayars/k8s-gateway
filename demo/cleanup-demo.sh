#!/bin/bash

# Cleanup script for gateway demo

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${YELLOW}=================================="
echo "Gateway Demo Cleanup"
echo "==================================${NC}"
echo ""

read -p "This will delete all demo resources. Continue? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Cleanup cancelled"
    exit 1
fi

echo ""
echo -e "${YELLOW}Step 1: Deleting gateway deployment${NC}"
kubectl delete -f ../k8s/deployment.yaml 2>/dev/null || echo "Gateway deployment not found"

echo ""
echo -e "${YELLOW}Step 2: Deleting gateway configuration${NC}"
kubectl delete configmap gateway-config 2>/dev/null || echo "Gateway ConfigMap not found"
kubectl delete secret gateway-ssh-keys 2>/dev/null || echo "Gateway SSH keys not found"

echo ""
echo -e "${YELLOW}Step 3: Deleting SR Linux topology${NC}"
kubectl delete -f clabernetes-topology-containerlab.yaml 2>/dev/null || echo "Topology not found"

echo ""
echo -e "${YELLOW}Step 4: Waiting for pods to terminate...${NC}"
kubectl wait --for=delete pod -l app=gateway --timeout=60s 2>/dev/null || true
kubectl wait --for=delete pod -l topology=srlinux-demo --timeout=60s 2>/dev/null || true

echo ""
echo -e "${YELLOW}Step 5: Cleaning up temporary files${NC}"
rm -f /tmp/ssh_host_key /tmp/ssh_host_key.pub
rm -f /tmp/client_key /tmp/client_key.pub
rm -f /tmp/authorized_keys
rm -f /tmp/gateway-config-updated.yaml
rm -f /tmp/demo-connection-info.txt

echo ""
echo -e "${GREEN}=================================="
echo "✓ Cleanup Complete!"
echo "==================================${NC}"
echo ""

echo "Remaining resources:"
kubectl get pods,svc,configmap,secrets | grep -E "gateway|srlinux|NAME" || echo "No demo resources remaining"
