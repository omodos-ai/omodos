.PHONY: all build install chunks clean

# --- Configurable Variables ---
# Change this date when you run your next export!
DATE ?= 2026-03-27

SOURCE_DIR ?= $(HOME)/ai/chats/source
CHUNKS_DIR ?= $(HOME)/ai/chats/chunks/$(DATE)
INGEST_DIR ?= ../ingest

all: build

build:
	@echo "Building ggo..."
	go build -o bin/ggo main.go

install: build
	@echo "Installing ggo to /usr/local/bin..."
	sudo install -m 755 bin/ggo /usr/local/bin/ggo

chunks: install
	@echo "\n======================================================="
	@echo " Running full Parse & Chunk pipeline for $(DATE)"
	@echo "=======================================================\n"
	ggo gemini-pr-resp $(SOURCE_DIR) $(INGEST_DIR)/gemini/$(DATE)-export
	ggo cg-pr-resp $(SOURCE_DIR) $(INGEST_DIR)/chatgpt/$(DATE)-export
	ggo chunks $(DATE) $(SOURCE_DIR) $(CHUNKS_DIR)

clean:
	rm -rf bin/
