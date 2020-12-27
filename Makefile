USE_CGO := 0
GIT_REVISION := $(shell git rev-parse --short HEAD)
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
GIT_VERSION := dev

default: build

.PHONY: all build clean lint test

build:
	env CGO_ENABLED=$(USE_CGO) go build -ldflags="-X 'main.Branch=$(GIT_BRANCH)' -X 'main.Revision=$(GIT_REVISION)' -X 'main.Version=$(GIT_VERSION)'"

clean:
	rm -f roger

lint:
	golangci-lint run

test:
	go test
