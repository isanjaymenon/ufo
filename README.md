# UFO — Username Finding Object

[![CI](https://github.com/isanjaymenon/ufo/actions/workflows/ci.yml/badge.svg)](https://github.com/isanjaymenon/ufo/actions/workflows/ci.yml)
[![Release](https://github.com/isanjaymenon/ufo/actions/workflows/release.yml/badge.svg)](https://github.com/isanjaymenon/ufo/releases)

Find a username across social networks by checking a list of site URL
templates. Existence is detected per site using HTTP status, error-message
substrings, or redirect behavior — not “HTTP 200 means the account exists.”

## Install

Download a release archive for your OS from
[GitHub Releases](https://github.com/isanjaymenon/ufo/releases). Each archive
contains the `ufo` binary and `urls.json`. Extract and run from that directory:

```bash
./ufo -u <username>
```

Windows:

```text
ufo.exe -u <username>
```

From source:

```bash
go install github.com/isanjaymenon/ufo/cmd/ufo@latest
```

`go install` does not include `urls.json`. Copy it from this repository (or
pass `-f`) so the binary can load the site list.

```bash
git clone https://github.com/isanjaymenon/ufo.git
cd ufo
go run ./cmd/ufo -u <username>
```

## Usage

```bash
ufo -u <username> [-f <url_file>] [-c <concurrency>] [-o <output_file>] [-v]
```

| Flag | Default | Description |
|---|---|---|
| `-u` | — | Username to search for (required) |
| `-f` | `urls.json` | JSON array of URL templates and/or site objects. Looks in the current directory, then next to the binary. |
| `-c` | `10` | Number of concurrent requests |
| `-o` | — | Output file for results (optional) |
| `-v` | off | Print request errors to stderr |
| `-version` | — | Print version |
| `-h` | — | Show help |

Hits are printed as profile URLs, one per line. Ctrl+C cancels in-flight work.

## Site list

`urls.json` is a JSON array. Each entry is either a URL template string with a
`$USERNAME` placeholder, or an object that says how to tell a real profile from
a missing user:

```json
[
  "https://$USERNAME.example.com",
  {
    "name": "Example",
    "url": "https://example.com/$USERNAME",
    "errorType": "status_code"
  },
  {
    "name": "Soft 404",
    "url": "https://social.example/u/$USERNAME",
    "errorType": "message",
    "errorMsg": ["User not found", "This account doesn’t exist"]
  },
  {
    "name": "Redirect on miss",
    "url": "https://profiles.example/$USERNAME",
    "errorType": "response_url"
  }
]
```

| Field | Meaning |
|---|---|
| `url` | Profile URL template (`$USERNAME` placeholder) |
| `urlProbe` | Optional URL requested for detection (API); hits still report `url` |
| `errorType` | `status_code` (default), `message`, `response_url`, or an array of those |
| `errorMsg` | Substring(s) that mean “not found” when `errorType` is `message` |
| `errorCode` | Status code(s) that mean “not found” when `errorType` is `status_code` |
| `regexCheck` | If set, usernames that do not match are skipped |
| `requestMethod` / `request_method` | `GET`, `HEAD`, `POST`, or `PUT` |
| `requestPayload` / `request_payload` | JSON body; `$USERNAME` is interpolated |
| `headers` | Extra request headers |
| `noRedirect` | Do not follow HTTP redirects |

Bare strings are `status_code` probes that **do not** follow redirects, so a
missing profile that 302s to a 200 homepage is not counted as a hit.

`status_code` counts an account as found only on HTTP 2xx (and not in
`errorCode`). `message` looks for `errorMsg` in the body and ignores non-2xx
responses (they are not hits). `response_url` treats a 2xx without a redirect
as found.

## Development

```bash
go test ./...
go vet ./...
gofmt -l .
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for site-list changes and how maintainers
cut a GitHub Release (`git tag -a v0.1.0 && git push origin v0.1.0`).

## License

[MIT](LICENSE)
