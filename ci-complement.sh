#!/bin/sh

git clone --depth=1 https://github.com/matrix-org/complement.git CI_COMPLEMENT
cd CI_COMPLEMENT
git fetch origin pull/14/head:complement-with-timeout
git checkout complement-with-timeout
go test -run '^(TestMediaWithoutFileName)$' -v ./tests
