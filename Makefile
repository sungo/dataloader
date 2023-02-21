all: staticcheck test

test:
	go test ./...

staticcheck:
	staticcheck ./...

example: test staticcheck
	go build -o loader_example example/main.go
