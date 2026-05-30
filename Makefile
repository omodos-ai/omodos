.PHONY: all build install chunks clean test tidy fmt vet proto

# 'all' now runs the complete hygiene and test pipeline before building
all: proto tidy fmt vet proto build

proto:
	@echo "Compiling Protobuf schemas..."
	protoc --go_out=. --go_opt=module=github.com/peterjbraam/ggo \
		-I proto proto/ggo/schema/v1/*.proto

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

build: proto
	@echo "Building master binary..."
	go build -o bin/ggo main.go
	@echo "Creating tool symlinks in ./bin..."
	ln -sf ggo bin/raggo

install: build
	@echo "Installing master binary to /usr/local/bin..."
	sudo install -m 755 bin/ggo /usr/local/bin/ggo
	@echo "Installing symlinks..."
	sudo ln -sf /usr/local/bin/ggo /usr/local/bin/raggo

clean:
	@echo "Cleaning build directory..."
	rm -rf bin/ pb/
