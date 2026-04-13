APP=papermap

.PHONY: build run test fmt

build:
	mkdir -p bin && go build -o ./bin/$(APP) ./cmd/papermap

run:
	go run ./cmd/papermap

test:
	go test ./...

fmt:
	gofmt -w cmd internal
