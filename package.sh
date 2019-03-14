#!/bin/bash
set -e

LDFLAGS="-X github.com/sunliver/shark-go/cmd.Version=`(git describe --tags --dirty || echo 'dev')`"

GOOS="linux" GOARCH="amd64" go build -ldflags "${LDFLAGS}" -o shark_linux_amd64
GOOS="linux" GOARCH="386" go build -ldflags "${LDFLAGS}" -o shark_linux_i386
GOOS="windows" GOARCH="amd64" go build -ldflags "${LDFLAGS}" -o shark_windows_amd64
GOOS="windows" GOARCH="386" go build -ldflags "${LDFLAGS}" -o shark_windows_i386
GOOS="darwin" GOARCH="amd64" go build -ldflags "${LDFLAGS}" -o shark_darwin_amd64
GOOS="darwin" GOARCH="386" go build -ldflags "${LDFLAGS}" -o shark_darwin_i386

zip -q -r shark_linux_amd64.zip shark_linux_amd64
zip -q -r shark_linux_i386.zip shark_linux_i386
zip -q -r shark_windows_amd64.zip shark_windows_amd64
zip -q -r shark_windows_i386.zip shark_windows_i386
zip -q -r shark_darwin_amd64.zip shark_darwin_amd64
zip -q -r shark_darwin_i386.zip shark_darwin_i386
rm shark_linux_amd64 shark_linux_i386 shark_windows_amd64 shark_windows_i386 shark_darwin_amd64 shark_darwin_i386
