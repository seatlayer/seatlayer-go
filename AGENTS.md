# Working in this repo

This is the SeatLayer **server** SDK. It talks to the API with a secret key and must never
be usable from a browser.

## Rules

- **No dependencies.** Standard library only: `net/http`, `encoding/json`, `crypto/hmac`. Keep it that way —
  a server SDK that drags in a dependency tree is a supply-chain surface for every customer.
- **The public surface is defined upstream** by `workers/api/src/publicApi.ts` in the app repo.
  A method here must map to an operation listed there. Do not wrap internal routes.
- **Method names are the operationIds** from that manifest, exported in Go style. Renaming one is
  a breaking change across every SeatLayer server SDK, not just this package.
- **Go idioms win over cross-SDK symmetry** where they conflict: errors are values checked with
  `errors.As`, every call takes a `context.Context`, options are structs rather than long argument
  lists, and `All()` is a range-over-func iterator. Matching Go beats matching the TypeScript shape.
- **Ergonomics live here, not in the transport.** Things like "capabilities is required" and
  "expectedUpdatedAt is required" are deliberate divergences from the raw API; each one has a
  comment saying why.

## Checks

`gofmt -l .` (must be empty), `go vet ./...`, and `go test -race ./...`. CI runs all three on
Go 1.23 and 1.24.

## Releasing

There is no registry account. Go resolves modules straight from the repo, so a release is a
semver git tag (`v0.1.0`) — which is also why `go.mod`'s module path must stay exactly
`github.com/seatlayer/seatlayer-go`.
