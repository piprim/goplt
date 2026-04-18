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
# Generate from a local template directory into the current directory
goplt generate --template ./my-template

# Generate into a specific output directory
goplt generate --template ./my-template --output ./projects/myapp
```

`goplt` will open an interactive form, collect all variable values declared in
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
# Optional: create a named subdirectory in the output dir (ignored when --output is set).
target-dir = "{{.Name}}"

[variables]
# Text input — empty default means the field is required
name          = ""

# Text input with a default value
org-prefix    = "github.com/acme"

# Boolean confirm (yes/no)
with-connect  = true

# Select from a list — first item is the default
license       = ["MIT", "Apache-2.0", "BSD-3-Clause", "GPL-3.0"]

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

| TOML value | Kind | TUI widget | Notes |
|---|---|---|---|
| `""` or `"default"` | text | text input | Empty default = required field |
| `true` / `false` | bool | yes/no confirm | |
| `["A", "B", "C"]` | select | dropdown | First item = default |

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

## CLI reference

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
```

### Types

```go
type Manifest struct {
    Variables  []Variable
    Conditions map[string]string
    Hooks      Hooks
}

type Hooks struct {
    PostGenHooks PostGenHooks `mapstructure:"post-generate"`
}

type PostGenHooks []string

type Variable struct {
    Name    string       // PascalCase
    Default any          // string | bool | []string
    Kind    VariableKind // KindText | KindBool | KindChoiceString
}
```

---

## Roadmap

- `goplt init` — interactively scaffold a new `template.toml`
