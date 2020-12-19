use_cgo := 0

default: build

clean:
	rm -f roger

build:
	env CGO_ENABLED=$(use_cgo) go build

test:
	go test
