# Makefile for building Scriptling CLI

BIN_DIR = bin
CLI_DIR = scriptling-cli
LDFLAGS = -ldflags="-s -w"

.PHONY: clean build build-all build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64 build-windows-amd64 build-windows-arm64 test install

clean:
	rm -rf $(BIN_DIR)

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

build:
	cd $(CLI_DIR) && go build $(LDFLAGS) -o ../$(BIN_DIR)/scriptling .

build-all: clean $(BIN_DIR) build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64 build-windows-amd64 build-windows-arm64

build-linux-amd64: $(BIN_DIR)
	cd $(CLI_DIR) && GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o ../$(BIN_DIR)/scriptling-linux-amd64 .

build-linux-arm64: $(BIN_DIR)
	cd $(CLI_DIR) && GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o ../$(BIN_DIR)/scriptling-linux-arm64 .

build-darwin-amd64: $(BIN_DIR)
	cd $(CLI_DIR) && GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o ../$(BIN_DIR)/scriptling-darwin-amd64 .

build-darwin-arm64: $(BIN_DIR)
	cd $(CLI_DIR) && GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o ../$(BIN_DIR)/scriptling-darwin-arm64 .

build-windows-amd64: $(BIN_DIR)
	cd $(CLI_DIR) && GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o ../$(BIN_DIR)/scriptling-windows-amd64.exe .

build-windows-arm64: $(BIN_DIR)
	cd $(CLI_DIR) && GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o ../$(BIN_DIR)/scriptling-windows-arm64.exe .

test:
	go test ./...

install: build
	cp $(BIN_DIR)/scriptling /usr/local/bin/scriptling