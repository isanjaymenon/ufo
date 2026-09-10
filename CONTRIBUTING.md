# Contributing

## Development

```bash
go test ./...
go vet ./...
gofmt -l .
```

`gofmt -l .` should print nothing. Pull requests run the same checks in CI.

## Site list

`urls.json` is a JSON array of URL strings and/or site objects. Prefer an
object when a bare HTTP 2xx check is not enough:

```json
{
  "name": "Example",
  "url": "https://example.com/$USERNAME",
  "errorType": "message",
  "errorMsg": "User not found"
}
```

`errorType` may be `status_code`, `message`, `response_url`, or an array of
those. See the README for the full field list.

## Releasing

Maintainers publish GitHub Releases from a signed version tag. CI runs
GoReleaser on tags matching `v*`:

```bash
git tag -s v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

Before tagging, move the `[Unreleased]` section in `CHANGELOG.md` to a dated
version heading (for example `## [0.1.0] - 2026-09-10`).

Use [semantic versioning](https://semver.org/). Archives include `ufo`,
`urls.json`, `LICENSE`, `README.md`, and `CHANGELOG.md` for linux, windows,
and darwin (amd64 and arm64).
