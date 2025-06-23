build:
	go build -o bin/app cmd/main.go

run: build
	./bin/app

clean:
	rm -rf bin/*

test:
	go test -v ./... -count=1

.PHONY: build run clean test