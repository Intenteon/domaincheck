.PHONY: all build server cli clean install

BINARY_SERVER = domaincheck-server
BINARY_CLI = domaincheck

all: build

build: server cli

server:
	go build -o $(BINARY_SERVER) ./cmd/server

cli:
	go build -o $(BINARY_CLI) ./cmd/cli

install: build
	cp $(BINARY_SERVER) /usr/local/bin/
	cp $(BINARY_CLI) /usr/local/bin/

clean:
	rm -f $(BINARY_SERVER) $(BINARY_CLI)

run-server: server
	./$(BINARY_SERVER)

# Quick test
test: cli
	./$(BINARY_CLI) intentixdf.com
