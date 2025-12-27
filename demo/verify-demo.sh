#!/bin/bash

# Verification script for gateway demo
# Checks if all components are deployed and accessible

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

FAILED=0

echo -e "${YELLOW}=================================="
echo "Gateway Demo Verification"
echo "==================================${NC}"
echo ""

# Function to check status
check_status() {
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ $1${NC}"
        return 0
    else
        echo -e "${RED}✗ $1${NC}"
        FAILED=$((FAILED + 1))
        return 1
    fi
}

# Check SR Linux pods
echo -e "${YELLOW}Checking SR Linux Topology...${NC}"

SRL_PODS=$(kubectl get pods -l topology=srlinux-demo -o jsonpath='{.items[*].metadata.name}' 2>/dev/null)
if [ ! -z "$SRL_PODS" ]; then
    check_status "SR Linux pods found"
    kubectl get pods -l topology=srlinux-demo

    # Check if pods are running
    RUNNING=$(kubectl get pods -l topology=srlinux-demo -o jsonpath='{.items[*].status.phase}' | grep -o "Running" | wc -l)
    if [ "$RUNNING" -eq 2 ]; then
        check_status "Both SR Linux pods are running"
    else
        check_status "SR Linux pods are not all running"
    fi
else
    check_status "SR Linux pods found"
fi

echo ""

# Check SR Linux services
echo -e "${YELLOW}Checking SR Linux Services...${NC}"
SRL_SVC=$(kubectl get svc -l topology=srlinux-demo -o jsonpath='{.items[*].metadata.name}' 2>/dev/null)
if [ ! -z "$SRL_SVC" ]; then
    check_status "SR Linux services found"
    kubectl get svc -l topology=srlinux-demo
else
    check_status "SR Linux services found"
fi

echo ""

# Check Gateway deployment
echo -e "${YELLOW}Checking Gateway Deployment...${NC}"

kubectl get deployment gateway > /dev/null 2>&1
if check_status "Gateway deployment exists"; then
    kubectl get deployment gateway

    # Check if deployment is ready
    READY=$(kubectl get deployment gateway -o jsonpath='{.status.readyReplicas}' 2>/dev/null)
    if [ "$READY" -gt 0 ]; then
        check_status "Gateway deployment is ready"
    else
        check_status "Gateway deployment is ready"
    fi
fi

echo ""

# Check Gateway services
echo -e "${YELLOW}Checking Gateway Services...${NC}"

kubectl get svc gateway-grpc > /dev/null 2>&1
check_status "Gateway gRPC service exists"

kubectl get svc gateway-ssh > /dev/null 2>&1
check_status "Gateway SSH service exists"

echo ""
kubectl get svc | grep gateway

echo ""

# Check Gateway configuration
echo -e "${YELLOW}Checking Gateway Configuration...${NC}"

kubectl get configmap gateway-config > /dev/null 2>&1
check_status "Gateway ConfigMap exists"

kubectl get secret gateway-ssh-keys > /dev/null 2>&1
check_status "Gateway SSH keys secret exists"

echo ""

# Check connectivity from gateway to SR Linux
echo -e "${YELLOW}Checking Gateway to SR Linux Connectivity...${NC}"

GATEWAY_POD=$(kubectl get pods -l app=gateway -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ ! -z "$GATEWAY_POD" ]; then
    echo "Testing from gateway pod: $GATEWAY_POD"

    # Get SR Linux service names
    SRL1_SVC=$(kubectl get svc -l topology=srlinux-demo,node=srl1 -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    SRL2_SVC=$(kubectl get svc -l topology=srlinux-demo,node=srl2 -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

    if [ ! -z "$SRL1_SVC" ]; then
        kubectl exec $GATEWAY_POD -- nc -zv ${SRL1_SVC} 22 > /dev/null 2>&1
        check_status "Gateway can reach SRL1 SSH port"
    fi

    if [ ! -z "$SRL2_SVC" ]; then
        kubectl exec $GATEWAY_POD -- nc -zv ${SRL2_SVC} 22 > /dev/null 2>&1
        check_status "Gateway can reach SRL2 SSH port"
    fi
else
    echo -e "${RED}✗ Gateway pod not found${NC}"
    FAILED=$((FAILED + 1))
fi

echo ""

# Check SR Linux is accessible
echo -e "${YELLOW}Checking SR Linux Access...${NC}"

SRL1_POD=$(kubectl get pods -l topology=srlinux-demo,node=srl1 -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ ! -z "$SRL1_POD" ]; then
    kubectl exec $SRL1_POD -- sr_cli "show version" > /dev/null 2>&1
    check_status "SR Linux 1 is accessible and responding"
else
    echo -e "${RED}✗ SR Linux 1 pod not found${NC}"
    FAILED=$((FAILED + 1))
fi

SRL2_POD=$(kubectl get pods -l topology=srlinux-demo,node=srl2 -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ ! -z "$SRL2_POD" ]; then
    kubectl exec $SRL2_POD -- sr_cli "show version" > /dev/null 2>&1
    check_status "SR Linux 2 is accessible and responding"
else
    echo -e "${RED}✗ SR Linux 2 pod not found${NC}"
    FAILED=$((FAILED + 1))
fi

echo ""

# Summary
echo -e "${YELLOW}==================================${NC}"
if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All checks passed!${NC}"
    echo ""
    echo "Demo is ready for testing!"
    echo ""
    echo "Next steps:"
    echo "1. Setup port forwarding:"
    echo "   kubectl port-forward svc/gateway-grpc 50051:50051 &"
    echo "   kubectl port-forward svc/gateway-ssh 2222:22 &"
    echo ""
    echo "2. Run tests:"
    echo "   ./test-gateway.sh"
else
    echo -e "${RED}✗ $FAILED check(s) failed${NC}"
    echo ""
    echo "Please review the errors above and try deploying again."
    echo "To redeploy: ./deploy-demo.sh"
fi
echo -e "${YELLOW}==================================${NC}"

exit $FAILED
