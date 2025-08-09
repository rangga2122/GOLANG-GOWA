# GOWA Broadcast Makefile

# Variables
APP_NAME=gowa-broadcast
DOCKER_IMAGE=$(APP_NAME):latest
DOCKER_CONTAINER=$(APP_NAME)
GO_VERSION=1.21
PORT=8080

# Colors for output
RED=\033[0;31m
GREEN=\033[0;32m
YELLOW=\033[1;33m
BLUE=\033[0;34m
NC=\033[0m # No Color

.PHONY: help build run test clean docker-build docker-run docker-stop docker-clean setup dev prod logs

# Default target
help: ## Show this help message
	@echo "$(BLUE)GOWA Broadcast - Available Commands:$(NC)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "$(GREEN)%-20s$(NC) %s\n", $$1, $$2}'

# Development
setup: ## Setup development environment
	@echo "$(YELLOW)Setting up development environment...$(NC)"
	@if [ ! -f .env ]; then cp .env.example .env; echo "$(GREEN)Created .env file from .env.example$(NC)"; fi
	@mkdir -p data
	@echo "$(GREEN)Development environment setup complete!$(NC)"

deps: ## Download dependencies
	@echo "$(YELLOW)Downloading dependencies...$(NC)"
	go mod download
	go mod tidy
	@echo "$(GREEN)Dependencies downloaded!$(NC)"

build: ## Build the application
	@echo "$(YELLOW)Building application...$(NC)"
	go build -o bin/$(APP_NAME) .
	@echo "$(GREEN)Build complete! Binary: bin/$(APP_NAME)$(NC)"

run: ## Run the application locally
	@echo "$(YELLOW)Starting application...$(NC)"
	go run main.go

dev: setup deps ## Setup and run in development mode
	@echo "$(YELLOW)Starting in development mode...$(NC)"
	SERVER_DEBUG=true go run main.go

test: ## Run tests
	@echo "$(YELLOW)Running tests...$(NC)"
	go test -v ./...

test-coverage: ## Run tests with coverage
	@echo "$(YELLOW)Running tests with coverage...$(NC)"
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report generated: coverage.html$(NC)"

clean: ## Clean build artifacts
	@echo "$(YELLOW)Cleaning build artifacts...$(NC)"
	rm -rf bin/
	rm -f coverage.out coverage.html
	@echo "$(GREEN)Clean complete!$(NC)"

# Docker commands
docker-build: ## Build Docker image
	@echo "$(YELLOW)Building Docker image...$(NC)"
	docker build -t $(DOCKER_IMAGE) .
	@echo "$(GREEN)Docker image built: $(DOCKER_IMAGE)$(NC)"

docker-run: ## Run application in Docker container
	@echo "$(YELLOW)Starting Docker container...$(NC)"
	docker run -d \
		--name $(DOCKER_CONTAINER) \
		-p $(PORT):$(PORT) \
		-v $$(pwd)/data:/app/data \
		-v $$(pwd)/.env:/app/.env:ro \
		$(DOCKER_IMAGE)
	@echo "$(GREEN)Container started: $(DOCKER_CONTAINER)$(NC)"
	@echo "$(BLUE)Application available at: http://localhost:$(PORT)$(NC)"

docker-stop: ## Stop Docker container
	@echo "$(YELLOW)Stopping Docker container...$(NC)"
	docker stop $(DOCKER_CONTAINER) || true
	docker rm $(DOCKER_CONTAINER) || true
	@echo "$(GREEN)Container stopped and removed$(NC)"

docker-logs: ## Show Docker container logs
	@echo "$(YELLOW)Showing container logs...$(NC)"
	docker logs -f $(DOCKER_CONTAINER)

docker-shell: ## Access Docker container shell
	@echo "$(YELLOW)Accessing container shell...$(NC)"
	docker exec -it $(DOCKER_CONTAINER) /bin/sh

docker-clean: ## Clean Docker images and containers
	@echo "$(YELLOW)Cleaning Docker artifacts...$(NC)"
	docker stop $(DOCKER_CONTAINER) || true
	docker rm $(DOCKER_CONTAINER) || true
	docker rmi $(DOCKER_IMAGE) || true
	docker system prune -f
	@echo "$(GREEN)Docker cleanup complete!$(NC)"

# Docker Compose commands
compose-up: ## Start services with Docker Compose
	@echo "$(YELLOW)Starting services with Docker Compose...$(NC)"
	docker-compose up -d
	@echo "$(GREEN)Services started!$(NC)"
	@echo "$(BLUE)Application available at: http://localhost:$(PORT)$(NC)"

compose-down: ## Stop services with Docker Compose
	@echo "$(YELLOW)Stopping services...$(NC)"
	docker-compose down
	@echo "$(GREEN)Services stopped!$(NC)"

compose-logs: ## Show Docker Compose logs
	@echo "$(YELLOW)Showing service logs...$(NC)"
	docker-compose logs -f

compose-build: ## Build services with Docker Compose
	@echo "$(YELLOW)Building services...$(NC)"
	docker-compose build
	@echo "$(GREEN)Services built!$(NC)"

compose-restart: ## Restart services
	@echo "$(YELLOW)Restarting services...$(NC)"
	docker-compose restart
	@echo "$(GREEN)Services restarted!$(NC)"

# Production deployment
prod-deploy: docker-build ## Deploy to production
	@echo "$(YELLOW)Deploying to production...$(NC)"
	@echo "$(RED)Make sure to update .env with production settings!$(NC)"
	docker-compose -f docker-compose.yml up -d
	@echo "$(GREEN)Production deployment complete!$(NC)"

prod-update: ## Update production deployment
	@echo "$(YELLOW)Updating production deployment...$(NC)"
	docker-compose pull
	docker-compose up -d
	@echo "$(GREEN)Production update complete!$(NC)"

# Utility commands
status: ## Show application status
	@echo "$(YELLOW)Checking application status...$(NC)"
	@if docker ps | grep -q $(DOCKER_CONTAINER); then \
		echo "$(GREEN)✓ Container is running$(NC)"; \
		curl -s -o /dev/null -w "HTTP Status: %{http_code}\n" http://localhost:$(PORT)/api/whatsapp/status || echo "$(RED)✗ Application not responding$(NC)"; \
	else \
		echo "$(RED)✗ Container is not running$(NC)"; \
	fi

logs: ## Show application logs
	@if docker ps | grep -q $(DOCKER_CONTAINER); then \
		make docker-logs; \
	else \
		echo "$(RED)Container is not running$(NC)"; \
	fi

backup: ## Backup application data
	@echo "$(YELLOW)Creating backup...$(NC)"
	@mkdir -p backups
	@tar -czf backups/gowa-backup-$$(date +%Y%m%d-%H%M%S).tar.gz data/ .env
	@echo "$(GREEN)Backup created in backups/ directory$(NC)"

restore: ## Restore from backup (usage: make restore BACKUP=filename)
	@if [ -z "$(BACKUP)" ]; then \
		echo "$(RED)Please specify backup file: make restore BACKUP=filename$(NC)"; \
		exit 1; \
	fi
	@echo "$(YELLOW)Restoring from backup: $(BACKUP)$(NC)"
	@tar -xzf backups/$(BACKUP)
	@echo "$(GREEN)Restore complete!$(NC)"

# Health checks
health: ## Check application health
	@echo "$(YELLOW)Checking application health...$(NC)"
	@curl -s http://localhost:$(PORT)/api/whatsapp/status | jq . || echo "$(RED)Health check failed$(NC)"

qr: ## Get WhatsApp QR code
	@echo "$(YELLOW)Getting WhatsApp QR code...$(NC)"
	@curl -s http://localhost:$(PORT)/api/whatsapp/qr | jq -r '.qr_code' || echo "$(RED)Failed to get QR code$(NC)"

# Development helpers
format: ## Format Go code
	@echo "$(YELLOW)Formatting code...$(NC)"
	go fmt ./...
	@echo "$(GREEN)Code formatted!$(NC)"

lint: ## Run linter
	@echo "$(YELLOW)Running linter...$(NC)"
	golangci-lint run
	@echo "$(GREEN)Linting complete!$(NC)"

mod-update: ## Update Go modules
	@echo "$(YELLOW)Updating Go modules...$(NC)"
	go get -u ./...
	go mod tidy
	@echo "$(GREEN)Modules updated!$(NC)"

# Quick commands
quick-start: setup deps docker-build docker-run ## Quick start (setup + build + run)
	@echo "$(GREEN)🚀 GOWA Broadcast is ready!$(NC)"
	@echo "$(BLUE)📱 Access the application at: http://localhost:$(PORT)$(NC)"
	@echo "$(BLUE)📋 Get QR code: make qr$(NC)"
	@echo "$(BLUE)📊 Check status: make status$(NC)"

quick-stop: docker-stop ## Quick stop
	@echo "$(GREEN)Application stopped!$(NC)"

restart: docker-stop docker-run ## Restart application
	@echo "$(GREEN)Application restarted!$(NC)"