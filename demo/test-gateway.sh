#!/bin/bash

# Test script for gateway demo
# This script tests various gateway functionalities

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

GATEWAY_GRPC="localhost:50051"
CLIENT_KEY="/tmp/client_key"
USERNAME="admin"
PASSWORD="admin"

echo -e "${YELLOW}=================================="
echo "Gateway Functionality Tests"
echo "==================================${NC}"
echo ""

# Check if port-forward is needed
if ! nc -z localhost 50051 2>/dev/null; then
    echo -e "${YELLOW}gRPC port not accessible. Please run:${NC}"
    echo "  kubectl port-forward svc/gateway-grpc 50051:50051"
    echo ""
    read -p "Press Enter after setting up port-forward..."
fi

# Test 1: SSH command to srl1
echo -e "${YELLOW}Test 1: Execute SSH command on srl1${NC}"
cd ../examples
go run grpc_client.go \
    -server ${GATEWAY_GRPC} \
    -fqdn srl1.customer.safabayar.net \
    -username ${USERNAME} \
    -password ${PASSWORD} \
    -command "show version" \
    -protocol ssh

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Test 1 passed${NC}"
else
    echo -e "${RED}✗ Test 1 failed${NC}"
fi
echo ""

# Test 2: SSH command to srl2
echo -e "${YELLOW}Test 2: Execute SSH command on srl2${NC}"
go run grpc_client.go \
    -server ${GATEWAY_GRPC} \
    -fqdn srl2.customer.safabayar.net \
    -username ${USERNAME} \
    -password ${PASSWORD} \
    -command "show version" \
    -protocol ssh

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Test 2 passed${NC}"
else
    echo -e "${RED}✗ Test 2 failed${NC}"
fi
echo ""

# Test 3: Get interface information
echo -e "${YELLOW}Test 3: Get interface information from srl1${NC}"
go run grpc_client.go \
    -server ${GATEWAY_GRPC} \
    -fqdn srl1.customer.safabayar.net \
    -username ${USERNAME} \
    -password ${PASSWORD} \
    -command "show interface brief" \
    -protocol ssh

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Test 3 passed${NC}"
else
    echo -e "${RED}✗ Test 3 failed${NC}"
fi
echo ""

# Test 4: Get network instance information
echo -e "${YELLOW}Test 4: Get network instance from srl2${NC}"
go run grpc_client.go \
    -server ${GATEWAY_GRPC} \
    -fqdn srl2.customer.safabayar.net \
    -username ${USERNAME} \
    -password ${PASSWORD} \
    -command "show network-instance" \
    -protocol ssh

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Test 4 passed${NC}"
else
    echo -e "${RED}✗ Test 4 failed${NC}"
fi
echo ""

# Test 5: NETCONF get-config (if NETCONF is available)
echo -e "${YELLOW}Test 5: NETCONF get-config on srl1${NC}"
go run grpc_client.go \
    -server ${GATEWAY_GRPC} \
    -fqdn srl1.customer.safabayar.net \
    -username ${USERNAME} \
    -password ${PASSWORD} \
    -command "<get-config><source><running/></source></get-config>" \
    -protocol netconf

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Test 5 passed${NC}"
else
    echo -e "${RED}✗ Test 5 failed (NETCONF may not be fully configured)${NC}"
fi
echo ""

echo -e "${GREEN}=================================="
echo "Tests Complete!"
echo "==================================${NC}"
