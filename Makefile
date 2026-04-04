.PHONY: all build install chunks clean test tidy fmt vet

# --- Configurable Variables ---
# Change this date when you run your next export!
DATE ?= 2026-03-27

SOURCE_DIR ?= $(HOME)/ai/chats/source
CHUNKS_DIR ?= $(HOME)/ai/chats/chunks/$(DATE)
INGEST_DIR ?= ../ingest

# 'all' now runs the complete hygiene and test pipeline before building
all: tidy fmt vet test build

tidy:
	@echo "Tidying module dependencies..."
	go mod tidy

fmt:
	@echo "Formatting Go code..."
	go fmt ./...

vet:
	@echo "Vetting Go source code..."
	go vet ./...

test:
	@echo "Running test suite..."
	go test -v ./...

build:
	@echo "Building ggo..."
	go build -o bin/ggo main.go

install: build
	@echo "Installing ggo to /usr/local/bin..."
	sudo install -m 755 bin/ggo /usr/local/bin/ggo

# Updated to use our new hyper-streamlined global batch pipeline
chunks: install
	@echo "\n======================================================="
	@echo " Running full Parse & Chunk pipeline for $(DATE)"
	@echo "=======================================================\n"
	ggo set-export-date $(DATE)
	ggo unzip
	ggo chunks

clean:
	@echo "Cleaning build directory..."
	rm -rf bin/
