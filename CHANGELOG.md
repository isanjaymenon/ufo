# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-09-10

### Added

- CLI (`ufo`) to look up a username across site URL templates in `urls.json`.
- Per-site existence detection: `status_code`, `message`, and `response_url`
  (JSON string or array), instead of treating HTTP 200 as “account exists.”
- Site objects in `urls.json` with `urlProbe`, `errorMsg`, `errorCode`,
  `regexCheck`, `headers`, `requestMethod` / `requestPayload`, and `noRedirect`.
- Bare URL strings treated as `status_code` probes that do not follow redirects.
- Concurrent checks (`-c`), optional result file (`-o`), verbose errors (`-v`),
  `-version`, and Ctrl+C cancellation.
- Default catalog lookup: `urls.json` in the current directory, then next to
  the executable (release archives ship the catalog beside the binary).
- HEAD probes for `status_code` sites, with GET fallback on HTTP 405.
- Skip WAF/challenge pages so they are not reported as hits.
- GitHub Actions CI, signed-tag GoReleaser builds (linux/windows/darwin,
  amd64/arm64), MIT license, and contribution docs.

[Unreleased]: https://github.com/isanjaymenon/ufo/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/isanjaymenon/ufo/releases/tag/v0.1.0
