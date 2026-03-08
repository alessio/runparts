#!/usr/bin/make -f

all: runparts check

runparts: generate
	go build

check: runparts
	go test -race ./...

generate: mod-tidy
	go generate ./internal/...

mod-tidy: go.mod
	go mod tidy

distclean: clean
clean:
	rm -fv runparts
	echo "v0.0.0-UNKNOWN" > ./internal/version/version.txt

.PHONY: all check clean distclean runparts mod-tidy generate
