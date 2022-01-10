.PHONY: build build-server build-client run-test-server

build: build-server build-client

build-server:
	go build ./cmd/wsp_server

build-client:
	go build ./cmd/wsp_client

run-test-server:
	go run ./examples/main.go
