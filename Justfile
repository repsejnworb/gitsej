set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

default: ci

fmt:
  rg --files -g '*.go' -0 | xargs -0 gofmt -w

fmt-check:
  test -z "$(rg --files -g '*.go' -0 | xargs -0 gofmt -l)"

build:
  go build ./...

test:
  go test ./...

ci: fmt-check build test
