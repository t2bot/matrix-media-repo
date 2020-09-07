#!/bin/sh

git clone --depth=1 https://github.com/matrix-org/complement.git CI_COMPLEMENT
cd CI_COMPLEMENT
go test -run '^(TestMediaWithoutFileName)$' -v ./tests
