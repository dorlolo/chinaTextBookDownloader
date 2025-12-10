# Makefile for downloader

# Binary name
BINARY=downloader

# Builds the project
build:
	go build -o ${BINARY} .

# Installs our project: copies binaries
install:
	go install .

# Cleans our project: deletes binaries
clean:
	if exist ${BINARY}.exe del ${BINARY}.exe
	if exist ${BINARY} del ${BINARY}

# Builds for Windows
build-windows:
	GOOS=windows GOARCH=amd64 go build -o ${BINARY}-windows-amd64.exe .

# Builds for Linux
build-linux:
	GOOS=linux GOARCH=amd64 go build -o ${BINARY}-linux-amd64 .

# Builds for macOS
build-macos:
	GOOS=darwin GOARCH=amd64 go build -o ${BINARY}-macos-amd64 .

# Cross-compilation for all platforms
build-all: build-windows build-linux build-macos

# Runs tests
test:
	go test -v ./...

# Runs go fmt against code
fmt:
	go fmt ./...

# Runs go vet against code
vet:
	go vet ./...

# Help
help:
	@echo "Usage: make [target]"
	@echo
	@echo "Targets:"
	@echo "  build           Builds the project"
	@echo "  install         Installs our project"
	@echo "  clean           Cleans our project"
	@echo "  build-windows   Builds for Windows"
	@echo "  build-linux     Builds for Linux"
	@echo "  build-macos     Builds for macOS"
	@echo "  build-all       Cross-compilation for all platforms"
	@echo "  test            Runs tests"
	@echo "  fmt             Formats code"
	@echo "  vet             Vets code"
	@echo "  help            Shows this help message"

.PHONY: build install clean build-windows build-linux build-macos build-all test fmt vet help