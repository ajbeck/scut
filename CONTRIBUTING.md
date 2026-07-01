# Contributing

Thanks for considering a contribution to `scut`.

## Development

This project targets Go 1.26 and uses `./walle` for all build, format, vet, and test commands.

```bash
./walle fmt
./walle test
./walle vet
./walle build
```

Do not run `go test`, `go build`, `go vet`, or `gofmt` directly for repository verification. Walle sets the required `GOEXPERIMENT=jsonv2` environment.

## Commits

Use Conventional Commits. Allowed commit types are:

- `feat`
- `patch`
- `docs`
- `refactor`
- `test`
- `chore`

## Documentation

Implementation documentation lives in `docs/` as HTML files. If a behavior change affects an existing docs page, update that page in the same pull request.

## Pull Requests

Before opening a pull request, run:

```bash
./walle fmt
./walle test
./walle vet
./walle build
```

Keep changes scoped and include tests for behavior changes.
