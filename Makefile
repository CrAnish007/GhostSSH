BINARY := gossh
BIN_DIR := bin
GO := go

.PHONY: all build clean

all: build

build:
	mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=amd64 $(GO) build -o $(BIN_DIR)/$(BINARY) .

clean:
	rm -rf $(BIN_DIR)

