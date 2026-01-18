BINARY := wing
BUILD_DIR := bin
VERSION := $(shell cat VERSION)
INSTALL_DIR ?= $(HOME)/.local/bin

.PHONY: build install test clean

build:
	@mkdir -p $(BUILD_DIR)
	go build -ldflags "-X main.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY) ./cmd/wing

install: build
	@mkdir -p $(INSTALL_DIR)
	cp $(BUILD_DIR)/$(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "Installed to $(INSTALL_DIR)/$(BINARY)"

test:
	go test ./...

clean:
	@rm -rf $(BUILD_DIR)
