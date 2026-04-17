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
post_generate = [
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
post_generate = [
  "go mod tidy",
  "git init",
  "git add .",
]
```

Commands are split on whitespace — no shell expansion, pipes, or redirections.
For complex logic, call a script instead:

```toml
post_generate = ["./scripts/post-gen.sh"]
```

---

## CLI reference

```
goplt generate [--template <dir>] [--output <dir>]
```

| Flag | Default | Description |
|---|---|---|
| `--template` | current directory | Directory containing `template.toml` |
| `--output` | current directory | Directory where files are written |

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
err = goplt.Generate(fsys, outputDir, vars)

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
    PostGenHooks PostGenHooks `mapstructure:"post_generate"`
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
- `--template <url>` — fetch a template from a remote Git repository
