set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

default: ci

fmt:
  git ls-files -z '*.go' | xargs -0 gofmt -w

fmt-check:
  test -z "$(git ls-files -z '*.go' | xargs -0 gofmt -l)"

build:
  go build ./...

test:
  go test ./...

ci: fmt-check build test
