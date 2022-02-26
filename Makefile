export GO111MODULE=on
export GOARCH=amd64
export GOOS=linux

.PHONY: build
build: bin/main

bin/main: main.go
	go build -o bin/main

.PHONY: deploy
deploy: bin/main
	lambroll deploy
