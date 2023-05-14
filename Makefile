all: staticcheck test

test:
	go test -v ./...

staticcheck:
	staticcheck ./...

example: test staticcheck
	go build -o loader_example example/main.go
