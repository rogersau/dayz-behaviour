.PHONY: test run fmt vet

test:
	go test ./...

run:
	go run ./cmd/ingestd

fmt:
	gofmt -w ./cmd ./internal ./pkg

vet:
	go vet ./...
