# Makefile for BG3 Guard (bg-guard)

APP_NAME := bg3-guard
LSLIB_VERSION := $(strip $(file < .lslib-version))

ifeq ($(OS),Windows_NT)
	BINARY := $(APP_NAME).exe
else
	BINARY := $(APP_NAME)
endif

.PHONY: all build test clean download-divine build-divine-source check-updates generate-icon

all: download-divine build

# Generate Windows application icon and syso resource files
generate-icon:
	@go run scripts/generate_icon.go

# Download and extract official precompiled divine.exe from Norbyte/lslib
download-divine:
	@go run scripts/download_divine.go $(LSLIB_VERSION)


# Compile divine from source using dotnet SDK if installed
build-divine-source:
	@echo "Cloning and building LSLib from source..."
	git clone --depth 1 https://github.com/Norbyte/lslib.git lslib_src
	dotnet build lslib_src/Divine/Divine.csproj -c Release -o assets/divine
	rm -rf lslib_src
	@echo "Divine compiled from source and copied to assets/divine"

build: download-divine
	@echo "Building $(BINARY)..."
	go build -ldflags="-s -w" -o $(BINARY) .
	@echo "Build complete: $(BINARY)"

test:
	go test -v ./...

clean:
	rm -rf $(BINARY) $(APP_NAME) export_tool_temp.zip assets

# Check for new versions of Go dependencies and LSLib Divine toolchain
check-updates:
	@go run scripts/check_updates.go $(LSLIB_VERSION)
