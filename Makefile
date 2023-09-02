#!/usr/bin/make -f

all: runparts check

runparts: generate
	go build ./...

check: runparts testdata run-tests.sh clean-testdata
	./run-tests.sh ./runparts testdata

generate: mod-tidy
	go generate ./internal/...

mod-tidy: go.mod
	go mod tidy

distclean: clean clean-testdata
clean:
	rm -f runparts

clean-testdata:
	find ./testdata/ -name 'gotStd*' -delete

.PHONY: all check clean distclean clean-testdata runparts mod-tidy generate
