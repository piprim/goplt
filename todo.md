# goplt — Missing Features

## Roadmap (explicitly planned)

- [ ] `goplt init` — interactively scaffold a new `template.toml` for template authors

## CLI gaps

- [ ] `goplt list` — list locally cached remote templates (from `$GOMODCACHE`)
- [ ] `--no-hooks` flag on `generate` — skip post-generation hooks without editing the manifest
- [ ] `--keep` flag on `test` — retain the generated output dir for debugging instead of cleaning up

## Variable system

- [ ] `KindChoiceInt` / numeric variables — integer values (e.g. `port = 8080`) currently produce a type error
- [ ] Hook env injection — expose collected variable values as env vars when running hooks (e.g. `$NAME`, `$ORG_PREFIX`)

## Hooks

- [ ] Pre-generation hooks (`pre-generate`) — run commands before files are written (e.g. dependency checks)

## Template collection

- [ ] Expand `github.com/piprim/goplt-tmpl` — add templates beyond `cli-cobra` (e.g. gRPC service, REST API, library)

## goplt init — future domains (v1 ships go-library-simple only)

- [ ] `go-library-multi` domain — library with multiple internal packages (`internal/auth/`, `internal/hash/`, etc.)
- [ ] `go-cli` domain — meta-template skeleton for CLI tools (Cobra, flags, version command)
- [ ] `go-service` domain — meta-template skeleton for HTTP/gRPC services
