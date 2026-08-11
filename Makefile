PATH := $(HOME)/go-sdk/go/bin:$(PATH)

.PHONY: run smoke test vet fmt

## run launches the TUI app
run:
	go run .

## smoke runs the integration check against a real MariaDB
smoke:
	go run ./cmd/smoke

## test runs the full test suite
test:
	go test ./...

## vet runs go vet across the module
vet:
	go vet ./...

## fmt checks gofmt cleanliness
fmt:
	test -z "$$(gofmt -l .)"
