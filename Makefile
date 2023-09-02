#!/usr/bin/make -f

all: runparts check

runparts: generate
	go build

check: runparts backup-testdata run-tests.sh
	./run-tests.sh ./runparts testdata

generate: mod-tidy
	go generate ./internal/...

mod-tidy: go.mod
	go mod tidy

distclean: clean restore-testdata
clean:
	rm -fv runparts

backup-testdata: testdata
	cp -a testdata backup-testdata

restore-testdata: backup-testdata
	rm -rfv testdata
	mv -v backup-testdata testdata

.PHONY: all check clean distclean runparts mod-tidy generate
