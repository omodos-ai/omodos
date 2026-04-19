.PHONY: all build install chunks clean test tidy fmt vet

# 'all' now runs the complete hygiene and test pipeline before building
all: tidy fmt vet build

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

clean:
	@echo "Cleaning build directory..."
	rm -rf bin/
