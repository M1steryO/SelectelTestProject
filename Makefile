PLUGIN_OUT ?= loglint.so

.PHONY: test build-plugin run

test:
	go test ./...

build-plugin:
	CGO_ENABLED=1 go build -buildmode=plugin -o $(PLUGIN_OUT) ./plugin/loglint.go

run:
	go run ./cmd/loglint ./...
