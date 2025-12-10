#!/bin/bash

# Build script for downloader

echo "Building downloader..."

# Detect OS
OS=$(uname -s)

# Build for the current platform
if [[ "$OS" == "Darwin" ]]; then
    echo "Building for macOS..."
    go build -o downloader-macos .
elif [[ "$OS" == "Linux" ]]; then
    echo "Building for Linux..."
    go build -o downloader-linux .
else
    echo "Building for unknown OS: $OS"
    go build -o downloader .
fi

if [ $? -eq 0 ]; then
    echo "Build successful!"
else
    echo "Build failed!"
    exit 1
fi