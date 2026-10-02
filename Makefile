BINARY := log-analyzer
BIN_DIR := bin

.PHONY: all build test vet fmt fmt-check check clean

all: check build

## build: compile the binary into bin/
build:
	go build -o $(BIN_DIR)/$(BINARY) .

## test: run all tests
test:
	go test ./...

## vet: run go vet
vet:
	go vet ./...

## fmt: format all Go files
fmt:
	gofmt -w .

## fmt-check: fail if any Go file is not gofmt-formatted
fmt-check:
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then echo "not gofmt-formatted:"; echo "$$unformatted"; exit 1; fi

## check: definition of done (formatting, vet, tests)
check: fmt-check vet test

## clean: remove build output
clean:
	rm -rf $(BIN_DIR)
