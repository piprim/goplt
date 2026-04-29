# goplt

Cookiecutter-style project scaffolding for Go. Renders a directory tree of
[`text/template`](https://pkg.go.dev/text/template) files driven by a
`template.toml` manifest, collecting variable values interactively via a TUI,
then running post-generation shell hooks.

---

## Install

```bash
go install github.com/piprim/goplt/cmd/goplt@latest
```

Or build from source:

```bash
git clone https://github.com/piprim/goplt
cd goplt
go build -o goplt ./cmd/goplt
```

---

## Quick start

```bash
# Scaffold a new template directory (for template authors)
goplt init --complexity minimal

# Generate from a local template directory into the current directory
goplt generate --template ./my-template

# Generate into a specific output directory
goplt generate --template ./my-template --output ./projects/myapp

# Test a template — generates with defaults, runs go build + go test in Docker
goplt test --template ./my-template

# Convert an existing Go project into a template
goplt templatize --output ../my-template
```

`goplt generate` will open an interactive form, collect all variable values declared in
`template.toml`, render the template tree, and run any post-generation hooks.

---

## Remote templates

`goplt` accepts a Go module reference as `--template`, using the same syntax as
`go get`. The module is fetched via the Go module proxy and cached locally in
`$GOMODCACHE` — subsequent runs are instant.

```bash
# Latest version
goplt generate --template github.com/piprim/goplt-tmpl/cli-cobra

# Pinned version
goplt generate --template github.com/piprim/goplt-tmpl/cli-cobra@v1.0.0

# Branch or commit
goplt generate --template github.com/piprim/goplt-tmpl/cli-cobra@main
```

The template module must contain a `template.toml` at its root.
Private modules are supported via the standard Go environment variables
(`GOPRIVATE`, `GONOSUMDB`, `GOFLAGS`, `GOAUTH`).

### Official template collection

[github.com/piprim/goplt-tmpl](https://github.com/piprim/goplt-tmpl) —
community templates maintained alongside `goplt`:

| Reference | Description |
|---|---|
| `github.com/piprim/goplt-tmpl/cli-cobra` | Go CLI with Cobra + Viper, structured logging, Makefile or mise |

---

## Template directory layout

A template directory contains a `template.toml` manifest and any number of
files (and subdirectories) to be rendered:

```
my-template/
  template.toml                  ← manifest (required)
  go.mod.tmpl                    ← .tmpl extension is stripped in output
  main.go                        ← rendered as-is (template syntax still applied)
  cmd/{{.Name}}/main.go          ← path itself is a template
  internal/
    {{.Name}}.go
```

### File name rendering

Both **file paths** and **file contents** are rendered as Go `text/template`
templates. Variable placeholders use the standard `{{.VarName}}` syntax.

The `.tmpl` extension is stripped from the output file name:
- `go.mod.tmpl` → `go.mod`
- `Makefile.tmpl` → `Makefile`

### Skipping template.toml

`template.toml` is never copied to the output directory.

---

## template.toml reference

```toml
# Required: one-line summary of what this template generates.
description = "A Go CLI scaffolded with Cobra and Viper"

# Optional: create a named subdirectory in the output dir (ignored when --output is set).
target-dir = "{{.Name}}"

[variables]
# Flat syntax — value determines the kind
name          = ""                                          # text, required
org-prefix    = "github.com/acme"                          # text with default
with-connect  = true                                       # bool confirm
license       = ["MIT", "Apache-2.0", "BSD-3-Clause"]     # select

# Sub-table syntax — adds description, explicit kind, and required flag
[variables.name]
kind        = "input"
required    = true
description = "Go module name, e.g. my-service"

[variables.org-prefix]
kind        = "input"
value       = "github.com/acme"
description = "Module path prefix, e.g. github.com/yourorg"

[variables.with-connect]
kind        = "bool"
value       = true
description = "Generate a Connect-RPC server"

[variables.license]
kind        = "stringChoice"
value       = ["MIT", "Apache-2.0", "BSD-3-Clause", "GPL-3.0"]
description = "License to apply to the project"

[variables.packages]
kind        = "stringList"
required    = true
description = "Internal package names"

[conditions]
# Skip a path prefix when the expression evaluates to an empty string.
# The key is an unrendered path prefix; the value is a Go template expression.
"internal/adapters/connect" = "{{if .WithConnect}}true{{end}}"
"contracts/proto"           = "{{if and .WithConnect .WithContract}}true{{end}}"

[hooks]
# Shell commands run sequentially in the output directory after generation.
# The first non-zero exit aborts the chain.
post-generate = [
  "go mod tidy",
  "go work sync",
]
```

### Variable types

| `kind` (sub-table) | Flat value | TUI widget | Notes |
|---|---|---|---|
| `"input"` | `""` or `"default text"` | Text input | `required = true` or empty flat value = mandatory |
| `"bool"` | `true` / `false` | Yes/No confirm | |
| `"stringChoice"` | `["A", "B", "C"]` | Dropdown | First item = selected by default |
| `"stringList"` | *(sub-table only)* | Comma-separated input | Value is `[]string` in templates |

Both syntaxes are equivalent — the sub-table form adds `description`, explicit `kind`, and `required`.
Flat syntax is shorthand for the three original kinds; `stringList` requires the sub-table form.

### Variable name normalisation

Variable names are normalised to **PascalCase** for use in templates.
You can write them in any style in `template.toml`:

| In template.toml | In templates |
|---|---|
| `with-connect` | `{{.WithConnect}}` |
| `with_connect` | `{{.WithConnect}}` |
| `withConnect` | `{{.WithConnect}}` |
| `org-prefix` | `{{.OrgPrefix}}` |

### target-dir

The optional `target-dir` field is a Go template expression evaluated against
the collected variable values. When present, `goplt` appends the rendered value
as a subdirectory of the output path — so files land in `<output>/<target-dir>`
instead of `<output>` directly.

```toml
target-dir = "{{.Name}}"
```

`target-dir` is **ignored** when `--output` is set explicitly on the command
line, giving the caller full control over the destination.

---

### Conditions

A condition maps a **path prefix** to a Go template expression. When the
expression evaluates to an empty string, the entire subtree rooted at that
prefix is skipped — no files under it are written to the output directory.

```toml
[conditions]
"cmd/migration" = "{{if .WithDatabase}}true{{end}}"
```

Conditions are evaluated against the collected variable values, so they can
combine multiple variables:

```toml
"contracts/proto" = "{{if and .WithContract .WithConnect}}true{{end}}"
```

### Loops

A loop generates one copy of a directory subtree for every item in a `stringList` variable. The `{{.item}}` placeholder is available in both paths and file contents during each iteration.

```toml
[variables.packages]
kind        = "stringList"
required    = true
description = "Internal package names"

[loops]
"internal/{{.item}}/" = ["Packages"]
```

With `Packages = ["auth", "hash"]` the generator produces `internal/auth/` and `internal/hash/`, rendering every file in the template subtree once per item.

**Loop variable in templates:**

```go
// internal/{{.item}}/{{.item}}.go — item is the current package name
package {{.item}}
```

**Combining with conditions:**

A condition on the parent prefix gates the entire loop:
```toml
[conditions]
"internal/" = "{{if .WithInternal}}true{{end}}"
```

A condition key that contains `{{.item}}` is evaluated per item:
```toml
[conditions]
"internal/{{.item}}/" = "{{if ne .item \"skip\"}}true{{end}}"
```

**Restrictions (v1):** only one variable name per loop entry — nested loops are not yet supported.

---

### Hooks

Post-generation hooks are shell commands that run in the **output directory**
after all files have been written. They are useful for steps that depend on the
generated files being present (e.g. `go mod tidy`).

```toml
[hooks]
post-generate = [
  "go mod tidy",
  "git init",
  "git add .",
]
```

Commands are split on whitespace — no shell expansion, pipes, or redirections.
For complex logic, call a script instead:

```toml
post-generate = ["./scripts/post-gen.sh"]
```

#### Security — hook confirmation

> **Warning:** hooks execute arbitrary shell commands on your machine.
> Only use templates from sources you trust.

When a template declares hooks, `goplt` prints each command and asks for
explicit confirmation before running anything:

```
⚠  WARNING: this template defines post-generate hooks that will run shell commands on your machine.
   Only proceed if you trust the template source.

   Commands that will be executed:
     • go mod tidy
     • git init
     • git add .

   Tip: to skip this prompt in CI or for trusted templates, re-run with:
     goplt generate --yes ...

? Run these hooks? [y/N]
```

Pass `--yes` (or `-y`) to bypass the prompt — useful in CI pipelines or when
working with your own trusted templates:

```bash
goplt generate --template ./my-template --yes
```

---

## Testing templates

`goplt test` validates a template end-to-end without manual input. It generates
the template using default variable values, then runs `go build ./...` and
`go test ./...` inside a Docker container — giving template authors a
CI-friendly smoke test.

```bash
# Test the template in the current directory
goplt test

# Test a specific template directory
goplt test --template ./my-template

# Test a remote template
goplt test --template github.com/piprim/goplt-tmpl/cli-cobra

# Use a specific Go image
goplt test --template ./my-template --image golang:1.23

# Collect variable values interactively instead of using defaults
goplt test --template ./my-template --ask
```

The generated files are piped as a tar archive to the container's stdin — no
volume mounts are needed. Post-generation hooks declared in `template.toml` run
inside the container before `go build` and `go test`.

**Default variable values used during test:**

| Kind | Default used |
|---|---|
| text with a default | the declared default value |
| text with no default (required) | the variable name itself (e.g. `Name`) |
| bool | the declared default |
| select | the first option |

**Requirements:** Docker must be installed and in `PATH`. The container needs
network access to download Go modules.

---

## Scaffolding templates

`goplt init` scaffolds a new template directory for template authors. It generates a
ready-to-use template skeleton — with a `template.toml`, Go source stubs, and optional
tooling files — using one of the built-in meta-templates.

```bash
# Scaffold a new template (opens an interactive form)
goplt init

# Pre-select a complexity level without going through that TUI question
goplt init --complexity standard

# Write the skeleton into a specific directory
goplt init --output ~/dev/my-new-template
```

### Domains

A **domain** selects which meta-template to scaffold from:

| Domain | Description |
|---|---|
| `go-library-simple` | Go library with optional linting, example tests, internal package, and toolchain scaffolding |

### Complexity levels

| Level | Files generated |
|---|---|
| `minimal` | `<Name>.go`, `<Name>_test.go`, `go.mod`, `README.md`, `.gitignore`, `template.toml` |
| `standard` | Adds `.golangci.yml` and an example test file (`<Name>_example_test.go`) |
| `advanced` | Adds an `internal/<Name>/` package and a `Makefile` or `mise.toml` (toolchain-dependent) |

### Toolchain (advanced only)

When `complexity = advanced` is selected, an additional question chooses the build toolchain:

| Toolchain | File generated |
|---|---|
| `make` | `Makefile.tmpl` |
| `mise` | `mise.toml.tmpl` |

---

## CLI reference

### `goplt init`

```
goplt init [--output <dir>] [--domain <name>] [--complexity <level>]
```

| Flag | Short | Default | Description |
|---|---|---|---|
| `--output` | `-o` | `<Name>` subdirectory in the current directory | Directory where the template skeleton is written |
| `--domain` | `-d` | `go-library-simple` | Meta-template domain to scaffold from |
| `--complexity` | `-c` | (interactive) | Pre-select the complexity level: `minimal`, `standard`, or `advanced` |

### `goplt generate`

```
goplt generate [--template <path|module>] [--output <dir>] [--yes]
```

| Flag | Short | Default | Description |
|---|---|---|---|
| `--template` | `-t` | current directory | Local path or Go module reference (`host/owner/repo[/subpath][@version]`) containing `template.toml` |
| `--output` | `-o` | current directory | Directory where files are written; when set, overrides `target-dir` declared in `template.toml` |
| `--yes` | `-y` | `false` | Skip the hook confirmation prompt |

**Safety:** the output directory cannot be the same as, or nested inside, the
template directory (and vice versa). `goplt` checks this before doing anything.

### `goplt test`

```
goplt test [--template <path|module>] [--image <docker-image>] [--ask]
```

| Flag | Short | Default | Description |
|---|---|---|---|
| `--template` | `-t` | current directory | Local path or Go module reference containing `template.toml` |
| `--image` | | `golang:latest` | Docker image to use for the build/test sandbox |
| `--ask` | | `false` | Collect variable values interactively instead of using defaults |

Exit code 0 on success, non-zero on failure — suitable for CI pipelines.

### `goplt templatize`

Converts an existing Go project into a reusable goplt template.

Reads `go.mod` in the source directory to auto-detect the project name and org
prefix. Opens a TUI to confirm the description, values, and which case forms to
substitute. Copies the project tree to `--output`, replacing all confirmed values
with template placeholders, and writes a ready-to-use `template.toml`.

```bash
# Automatic — reads go.mod, opens TUI
goplt templatize --output ../my-template

# CI mode — no TUI, substitutes all forms
goplt templatize --output ../my-template --yes

# Protect "application" from being replaced by "app" substitution
goplt templatize --output ../my-template --skip application
```

| Flag | Description |
|---|---|
| `-o`, `--output` | Output directory (required) |
| `-d`, `--dir` | Source directory (default: current directory) |
| `--name` | Override project name (default: last segment of module path) |
| `--org-prefix` | Override org prefix (default: everything before last segment) |
| `--skip` | Protect a string from substitution; repeatable |
| `-y`, `--yes` | Pre-answer all yes/no questions as "yes" (CI mode) |

---

## Using goplt as a library

The root package is importable for embedding in your own tooling:

```go
import "github.com/piprim/goplt"

// Load the manifest from any fs.FS (os.DirFS, embed.FS, fstest.MapFS, …)
m, err := goplt.LoadManifest(fsys)

// Render the template tree into outputDir using collected variables
err = goplt.Generate(fsys, m, outputDir, vars)

// Run post-generation hooks declared in the manifest
err = goplt.RunHooks(m, outputDir)

// Normalise a variable name to PascalCase
key := goplt.NormalizeKey("with-connect") // → "WithConnect"

// Access the built-in template function map (snake, camel, pascal, kebab, …)
fm := goplt.DefaultFuncMap()

// Add custom functions on top of the built-ins
g := goplt.NewGenerator().WithFuncs(template.FuncMap{"myFunc": myFunc})
err = g.Generate(fsys, m, outputDir, vars)
```

### Types

```go
type Manifest struct {
    Description string             // required one-line summary of what this template generates
    Variables  []Variable
    Conditions map[string]string   // path prefix → template boolean expression
    Hooks      Hooks
    TargetDir  string              // optional template expression for output subdirectory
    Delimiters [2]string           // action delimiters; default ["{{", "}}"]
    Loops      map[string][]string // path pattern → [varName]; varName must be KindListString
}

type Hooks struct {
    PostGenHooks PostGenHooks `mapstructure:"post-generate"`
}

type PostGenHooks []string

type Variable struct {
    Name        string       // PascalCase
    Kind        VariableKind // KindInput | KindBool | KindChoiceString | KindListString
    Value       any          // string (KindInput default) | bool (KindBool default) | []string (choices or suggestions)
    Required    bool         // KindInput/KindListString: validation fails on empty
    Description string       // optional; shown as subtitle in the TUI
}
```

### Built-in template functions

`DefaultFuncMap` provides these functions for use in template files:

| Function | Description | Example |
|---|---|---|
| `snake` | Convert to snake_case | `{{snake .Name}}` → `my_service` |
| `camel` | Convert to camelCase | `{{camel .Name}}` → `myService` |
| `pascal` | Convert to PascalCase | `{{pascal .Name}}` → `MyService` |
| `kebab` | Convert to kebab-case | `{{kebab .Name}}` → `my-service` |
| `upper` | UPPER CASE | `{{upper .Name}}` → `MY-SERVICE` |
| `lower` | lower case | `{{lower .Name}}` → `my-service` |
| `trim` | Strip leading/trailing whitespace | `{{trim .Name}}` |
| `replace` | Replace all occurrences | `{{replace "-" "_" .Name}}` |
| `hasPrefix` | String has prefix | `{{if hasPrefix "github" .OrgPrefix}}` |
| `hasSuffix` | String has suffix | `{{if hasSuffix ".io" .OrgPrefix}}` |
| `contains` | String contains substr | `{{if contains "acme" .OrgPrefix}}` |

---

## LLM policy

This project is in part assisted by LLMs.
