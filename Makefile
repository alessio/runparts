#!/usr/bin/make -f

all: runparts check

runparts:
	go build ./...

check: runparts testdata run-tests.sh clean-testdata
	./run-tests.sh ./runparts testdata

distclean: clean clean-testdata
clean:
	rm -f runparts

clean-testdata:
	find ./testdata/ -name 'gotStd*' -delete

.PHONY: all check clean distclean clean-testdata runparts
