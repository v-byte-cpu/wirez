#!/usr/bin/env sh
set -eu

unformatted="$(gofmt -l .)"
if [ -n "$unformatted" ]; then
	printf 'The following Go files are not formatted:\n%s\n' "$unformatted" >&2
	exit 1
fi

go test ./... -v -cover
go build -trimpath -ldflags "-w -s" -o /tmp/wirez
