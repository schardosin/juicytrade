# Variables
REGISTRY := registry.gitlab.com/schardosin/juicytrade
BACKEND_IMAGE := trade-backend
FRONTEND_IMAGE := trade-app
SIMULATION_IMAGE := strategy-simulation
STRATEGY_IMAGE := strategy-service
TAG ?= latest
PLATFORM ?= linux/amd64

# Targets
.PHONY: all build push build-backend push-backend run-backend build-frontend push-frontend run-frontend build-simulation push-simulation run-simulation build-strategy push-strategy run-strategy help

all: build ## Build all images

build: build-backend build-frontend build-simulation build-strategy ## Build all images

push: push-backend push-frontend push-simulation push-strategy ## Push all images to registry

# Backend (Core Trading - Go, port 8008)
build-backend: ## Build the backend Docker image
	@echo "Building backend image..."
	docker build --platform $(PLATFORM) --load -t $(REGISTRY)/$(BACKEND_IMAGE):$(TAG) ./trade-backend-go

push-backend: ## Push the backend Docker image to registry
	@echo "Pushing backend image..."
	docker push $(REGISTRY)/$(BACKEND_IMAGE):$(TAG)

run-backend: ## Run the Go backend locally (port 8008)
	@echo "Starting Go backend on port 8008..."
	cd trade-backend-go && go run cmd/server/main.go

# Frontend (Vue.js, port 3001)
build-frontend: ## Build the frontend Docker image
	@echo "Building frontend image..."
	docker build --platform $(PLATFORM) --load -t $(REGISTRY)/$(FRONTEND_IMAGE):$(TAG) ./trade-app

push-frontend: ## Push the frontend Docker image to registry
	@echo "Pushing frontend image..."
	docker push $(REGISTRY)/$(FRONTEND_IMAGE):$(TAG)

run-frontend: ## Run the frontend locally (port 3001)
	@echo "Starting frontend on port 3001..."
	cd trade-app && npm run dev

# Strategy Simulation Service (Go)
build-simulation: ## Build the strategy simulation Docker image
	@echo "Building strategy simulation image..."
	docker build --platform $(PLATFORM) --load -t $(REGISTRY)/$(SIMULATION_IMAGE):$(TAG) ./strategy-simulation-go

push-simulation: ## Push the strategy simulation Docker image to registry
	@echo "Pushing strategy simulation image..."
	docker push $(REGISTRY)/$(SIMULATION_IMAGE):$(TAG)

run-simulation: ## Run the strategy simulation service locally
	@echo "Starting strategy simulation service..."
	cd strategy-simulation-go && go run cmd/server/main.go

# Strategy Service (Python, port 8009 - handles strategies + data-import)
build-strategy: ## Build the strategy service Docker image
	@echo "Building strategy service image..."
	docker build --platform $(PLATFORM) --load -t $(REGISTRY)/$(STRATEGY_IMAGE):$(TAG) ./strategy-service

push-strategy: ## Push the strategy service Docker image to registry
	@echo "Pushing strategy service image..."
	docker push $(REGISTRY)/$(STRATEGY_IMAGE):$(TAG)

run-strategy: ## Run the strategy service locally (port 8009)
	@echo "Starting strategy service on port 8009..."
	cd strategy-service && python -m uvicorn src.main:app --host 0.0.0.0 --port 8009

# Development helpers
dev: ## Run all services locally for development (backend, frontend, strategy)
	@echo "Starting all services..."
	@echo "  - Go backend:       http://localhost:8008"
	@echo "  - Strategy service: http://localhost:8009"
	@echo "  - Frontend:         http://localhost:3001"
	@make -j3 run-backend run-strategy run-frontend

dev-all: ## Run all services including simulation
	@echo "Starting all services including simulation..."
	@make -j4 run-backend run-frontend run-simulation run-strategy

test-backend: ## Run tests for Go backend
	@echo "Running Go backend tests..."
	cd trade-backend-go && go test ./...

test-simulation: ## Run tests for strategy simulation service
	@echo "Running strategy simulation tests..."
	cd strategy-simulation-go && go test ./...

test-strategy: ## Run tests for strategy service
	@echo "Running strategy service tests..."
	cd strategy-service && python -m pytest

test: test-backend test-simulation test-strategy ## Run all tests

logs-strategy: ## Tail strategy service logs
	@echo "Tailing strategy service..."
	cd strategy-service && python -m uvicorn src.main:app --host 0.0.0.0 --port 8009 --reload

logs-backend: ## Run Go backend with hot reload (requires air)
	@echo "Starting Go backend with hot reload..."
	cd trade-backend-go && air

help: ## Display this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Services:"
	@echo "  trade-backend-go  - Go backend (port 8008) - main API gateway"
	@echo "  strategy-service  - Python service (port 8009) - strategies + data-import"
	@echo "  trade-app         - Vue.js frontend (port 3001)"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
