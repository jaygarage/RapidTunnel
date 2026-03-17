GO       := go
BIN_DIR  := dist
APP_NAME := rapidtunnel

# Detect platform
OS       := $(shell $(GO) env GOOS)
ARCH     := $(shell $(GO) env GOARCH)

EXT :=
ifeq ($(OS),windows)
EXT := .exe
endif

OUT := $(BIN_DIR)/$(APP_NAME)-$(OS)-$(ARCH)$(EXT)

# Default target
all: build

# Build the binary
build:
	@echo "==> Building $(APP_NAME) for $(OS)/$(ARCH)"
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(OUT) ./cmd/.
	@echo "==> Output: $(OUT)"

# Run the application
run:
	@echo "==> Running $(APP_NAME)"
	$(GO) run ./cmd/main.go

# Tidy modules
tidy:
	@echo "==> Tidying modules"
	$(GO) mod tidy

# Download dependencies
deps:
	@echo "==> Downloading dependencies"
	$(GO) mod download

# Run tests
test:
	@echo "==> Running tests"
	$(GO) test ./...

# Format code
fmt:
	@echo "==> Formatting code"
	$(GO) fmt ./...

# Clean build artifacts
clean:
	@echo "==> Cleaning dist directory"
	rm -rf $(BIN_DIR)

.PHONY: all build run tidy deps test fmt clean