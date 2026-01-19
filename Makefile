# Variables
REGISTRY := registry.gitlab.com/schardosin/juicytrade
BACKEND_IMAGE := trade-backend
FRONTEND_IMAGE := trade-app
SIMULATION_IMAGE := strategy-simulation
TAG ?= latest
PLATFORM ?= linux/amd64

# Targets
.PHONY: all build push build-backend push-backend run-backend build-frontend push-frontend run-frontend build-simulation push-simulation run-simulation help

all: build ## Build all images

build: build-backend build-frontend build-simulation ## Build all images

push: push-backend push-frontend push-simulation ## Push all images to registry

# Backend (Core Trading)
build-backend: ## Build the backend Docker image
	@echo "Building backend image..."
	docker build --platform $(PLATFORM) --load -t $(REGISTRY)/$(BACKEND_IMAGE):$(TAG) ./trade-backend-go

push-backend: ## Push the backend Docker image to registry
	@echo "Pushing backend image..."
	docker push $(REGISTRY)/$(BACKEND_IMAGE):$(TAG)

run-backend: ## Run the Go backend locally
	@echo "Starting Go backend..."
	cd trade-backend-go && go run cmd/server/main.go

# Frontend
build-frontend: ## Build the frontend Docker image
	@echo "Building frontend image..."
	docker build --platform $(PLATFORM) --load -t $(REGISTRY)/$(FRONTEND_IMAGE):$(TAG) ./trade-app

push-frontend: ## Push the frontend Docker image to registry
	@echo "Pushing frontend image..."
	docker push $(REGISTRY)/$(FRONTEND_IMAGE):$(TAG)

run-frontend: ## Run the frontend locally
	@echo "Starting frontend..."
	cd trade-app && npm run dev

# Strategy Simulation Service
build-simulation: ## Build the strategy simulation Docker image
	@echo "Building strategy simulation image..."
	docker build --platform $(PLATFORM) --load -t $(REGISTRY)/$(SIMULATION_IMAGE):$(TAG) ./strategy-simulation-go

push-simulation: ## Push the strategy simulation Docker image to registry
	@echo "Pushing strategy simulation image..."
	docker push $(REGISTRY)/$(SIMULATION_IMAGE):$(TAG)

run-simulation: ## Run the strategy simulation service locally
	@echo "Starting strategy simulation service..."
	cd strategy-simulation-go && go run cmd/server/main.go

# Development helpers
dev: ## Run all services locally for development
	@echo "Starting all services..."
	@make -j3 run-backend run-frontend run-simulation

test-simulation: ## Run tests for strategy simulation service
	@echo "Running strategy simulation tests..."
	cd strategy-simulation-go && go test ./...

help: ## Display this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
