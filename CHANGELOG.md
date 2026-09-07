# Changelog

All notable changes to this project will be documented in this file.

This project uses Conventional Commits and GitHub Releases.

## [0.9.0](https://github.com/ajbeck/scut/compare/v0.8.0...v0.9.0) (2026-09-07)


### Features

* **format:** add Markdown formatting options ([#70](https://github.com/ajbeck/scut/issues/70)) ([28dd03d](https://github.com/ajbeck/scut/commit/28dd03d087eff74b221d6a2a2388cf1088ac086d))
* **format:** format named files in place ([#69](https://github.com/ajbeck/scut/issues/69)) ([9bba1be](https://github.com/ajbeck/scut/commit/9bba1be1381f86d057a7aacd7f95a382d7a6c770))
* **format:** migrate Markdown renderer to v2 ([#67](https://github.com/ajbeck/scut/issues/67)) ([6df892f](https://github.com/ajbeck/scut/commit/6df892f1b406119442ab39e8c13d696cd13423cc))
* **gotools:** add an independent module archive cache ([#58](https://github.com/ajbeck/scut/issues/58)) ([e474f58](https://github.com/ajbeck/scut/commit/e474f58a5811922d384da0c06dd97343c3638811))
* **gotools:** add module cache management commands ([#60](https://github.com/ajbeck/scut/issues/60)) ([af4016f](https://github.com/ajbeck/scut/commit/af4016f432d3614685fd27d71f1a090921872543))
* **mcp:** upgrade AWS proxy to v1 ([#66](https://github.com/ajbeck/scut/issues/66)) ([f5b36b5](https://github.com/ajbeck/scut/commit/f5b36b5035ac3e5f38d6f469beff48ca0ba723ae))


### Bug Fixes

* **gotools:** honor active Go build context ([#63](https://github.com/ajbeck/scut/issues/63)) ([ea10959](https://github.com/ajbeck/scut/commit/ea109594476d3c515b0baa5992f6bf100e2f679f))
* **gotools:** honor Go module download policy ([#61](https://github.com/ajbeck/scut/issues/61)) ([c2ca987](https://github.com/ajbeck/scut/commit/c2ca987b3e4631215dd5ab0b19af08b248c6411a))
* **gotools:** reuse Go module archives read-only ([#59](https://github.com/ajbeck/scut/issues/59)) ([3ada42d](https://github.com/ajbeck/scut/commit/3ada42d397d8dd40ff5c9785125f66b1014754b1))
* **gotools:** stop mutating the Go module cache ([#57](https://github.com/ajbeck/scut/issues/57)) ([9122626](https://github.com/ajbeck/scut/commit/91226266917b82cd9855c4b37ed8d9f87eff9c33))
* **gotools:** verify module archive integrity ([#62](https://github.com/ajbeck/scut/issues/62)) ([4130e3a](https://github.com/ajbeck/scut/commit/4130e3a4378f711202708fbf86a5c9517d33ee14))


### Miscellaneous Chores

* **deps:** update Go module graph ([#68](https://github.com/ajbeck/scut/issues/68)) ([34052fd](https://github.com/ajbeck/scut/commit/34052fd7b9aca28c38837fcd9ae79797ee40d91d))
* **go:** upgrade to Go 1.27.1 ([#65](https://github.com/ajbeck/scut/issues/65)) ([693b5e5](https://github.com/ajbeck/scut/commit/693b5e5a5c92410eee51d9d43b1ddf36bcc089f0))
* **release:** adopt release please ([#72](https://github.com/ajbeck/scut/issues/72)) ([712f534](https://github.com/ajbeck/scut/commit/712f5346eb5183077c98e117d317b436b699fcca))

## [v0.1.0] - 2026-05-29

Initial public release.

### Added

- Claude Code hook command surface for all hook events.
- `PostToolUse` formatter for Go, Markdown, and MDX files.
- `.prettierignore` and `.scutignore` support for formatting.
- Claude Code settings installer, status, and uninstall commands.
- Claude Code status line.
- `gotools doc` command for agent-friendly Go documentation lookup.
- Structured JSONL logging and log cleanup command.
- GitHub Actions workflows for pull requests, builds, releases, and Pages deployment.
- Release installer script with checksum verification.

[v0.1.0]: https://github.com/ajbeck/scut/releases/tag/v0.1.0
