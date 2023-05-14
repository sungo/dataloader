all: staticcheck test

test:
	go test -v ./...

staticcheck:
	staticcheck ./...
