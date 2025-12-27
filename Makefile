.PHONY: help build run test clean proto docker-build docker-push k8s-deploy k8s-delete generate-keys

# Variables
APP_NAME=gateway
DOCKER_IMAGE=safabayar/gateway
DOCKER_TAG=latest
K8S_NAMESPACE=default

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

proto: ## Generate protobuf code
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/gateway.proto

build: ## Build the gateway binary
	go build -o bin/gateway ./cmd/gateway

run: ## Run the gateway locally
	go run ./cmd/gateway -config config/devices.yaml -log logs/gateway.log

test: ## Run tests
	go test -v ./...

clean: ## Clean build artifacts
	rm -rf bin/ logs/*.log

docker-build: ## Build Docker image
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

docker-push: ## Push Docker image to registry
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)

k8s-deploy: ## Deploy to Kubernetes
	kubectl apply -f k8s/configmap.yaml
	kubectl apply -f k8s/deployment.yaml
	kubectl apply -f k8s/gateway-api.yaml

k8s-delete: ## Delete from Kubernetes
	kubectl delete -f k8s/gateway-api.yaml
	kubectl delete -f k8s/deployment.yaml
	kubectl delete -f k8s/configmap.yaml

generate-keys: ## Generate SSH host key and authorized keys
	@echo "Generating SSH host key..."
	@mkdir -p config
	@ssh-keygen -t ed25519 -f config/ssh_host_key -N "" -C "gateway-host-key"
	@echo "Generating client SSH key pair..."
	@ssh-keygen -t ed25519 -f config/client_key -N "" -C "gateway-client-key"
	@cp config/client_key.pub config/authorized_keys
	@echo "Keys generated successfully!"
	@echo "Host key: config/ssh_host_key"
	@echo "Client key: config/client_key"
	@echo "Authorized keys: config/authorized_keys"

deps: ## Download dependencies
	go mod download
	go mod tidy

fmt: ## Format code
	go fmt ./...

lint: ## Run linter
	golangci-lint run ./...

.DEFAULT_GOAL := help
