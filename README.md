# loglint — linter for log messages (slog + zap)

This project implements a `go/analysis` analyzer and a **golangci-lint custom plugin** that validates log messages against the rules from the test task:

1) messages start with a lowercase letter  
2) messages are English-only  
3) messages contain no punctuation/symbols/emoji  
4) messages are not constructed in a way that may leak sensitive data

Supported loggers:
- `log/slog`
- `go.uber.org/zap`

---

## Quick start (standalone)

```bash
go run ./cmd/loglint ./...
```

## Quick start (golangci-lint plugin)

### 1) Build the plugin

> Go plugins require `CGO_ENABLED=1`, and the plugin must be built for the **same OS/ARCH/Go toolchain** (and often same deps) as `golangci-lint`.  
> See golangci-lint plugin docs.

```bash
CGO_ENABLED=1 go build -buildmode=plugin -o loglint.so ./plugin/loglint.go
```

### 2) Configure `.golangci.yml`

Example:

```yaml
linters-settings:
  custom:
    loglint:
      path: ./loglint.so
      description: Checks slog/zap log messages.
      settings:
        rules:
          lowercase_start: true
          english_only: true
          no_special: true
          no_sensitive: true
        sensitive:
          # used for matching identifier names and key-like prefixes in dynamic messages
          keywords: ["password", "passwd", "pwd", "token", "api_key", "apikey", "secret", "private_key", "authorization", "bearer", "session"]
        allowed:
          # allow_punct is false by default (strict). Turn on to allow '.', ',', ':', ';', '?'.
          allow_punct: false
```

Enable the linter (if you have disable-all):

```yaml
linters:
  enable:
    - loglint
```

Run:

```bash
golangci-lint run
```

---

## Settings

Settings come from the `settings:` field of your custom linter entry in `.golangci.yml`.  
Note: golangci-lint uses Viper, which normalizes keys to lowercase.

- `rules.*` — enable/disable rules
- `sensitive.keywords` — list of sensitive keywords
- `allowed.allow_punct` — relax punctuation rule

---

## Tests

```bash
go test ./...
```

Test sources are in `pkg/loglint/testdata/` and run via `analysistest`.
