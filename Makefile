BINARY=jkl
VERSION= $(shell (git describe --tags --dirty 2>/dev/null || echo dev) |cut -dv -f2)
GIT_COMMIT=$(shell git rev-parse HEAD)
LDFLAGS="-s -w -X github.com/ivanfetch/jkl.Version=$(VERSION) -X github.com/ivanfetch/jkl.GitCommit=$(GIT_COMMIT)"

all: build

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:go.sum
	go vet ./...

go.sum:go.mod
	go get -t github.com/ivanfetch/jkl

.PHONY: test
test:go.sum
	go test -coverprofile=cover.out

.PHONY: integrationtest
integrationtest:go.sum
	go test -tags integration -coverprofile=cover.out

.PHONY: binary
binary:go.sum
	go build -ldflags $(LDFLAGS) -o $(BINARY) cmd/jkl/main.go

.PHONY: build
build: fmt vet test binary
