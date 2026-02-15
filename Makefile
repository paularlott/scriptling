# Makefile for building Scriptling CLI

BIN_DIR = bin
CLI_DIR = scriptling-cli
BUILD_DATE = $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS = -ldflags="-s -w -X github.com/paularlott/scriptling/build.BuildDate=$(BUILD_DATE)"

.PHONY: clean build build-all build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64 build-windows-amd64 build-windows-arm64 test release

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

release: build-all
	# Check if tag exists
	if git tag -l v$(shell go run ./tools/getversion) | grep -q v$(shell go run ./tools/getversion); then \
		echo "Tag already exists, skipping tag creation"; \
	else \
		echo "Creating tag"; \
		git tag -a v$(shell go run ./tools/getversion) -m "Release $(shell go run ./tools/getversion)"; \
		git push origin v$(shell go run ./tools/getversion); \
	fi
	# Create release and upload all binaries
	gh release create v$(shell go run ./tools/getversion) -t "Release $(shell go run ./tools/getversion)" -n "Scriptling $(shell go run ./tools/getversion)" $(BIN_DIR)/*