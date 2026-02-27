BINARY := odh-chaos
PKG := github.com/opendatahub-io/odh-platform-chaos
CMD := ./cmd/odh-chaos

.PHONY: build test test-short lint clean install

build:
	go build -o bin/$(BINARY) $(CMD)

test:
	go test ./... -v -count=1

test-short:
	go test ./... -short -count=1

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/

install: build
	cp bin/$(BINARY) $(GOPATH)/bin/
