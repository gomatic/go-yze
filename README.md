# go-yze

The framework library for the [`yze`](https://github.com/gomatic/yze) analyzer family — the shared scaffolding that makes each `gomatic/yze-<group>-<name>` analyzer repo cheap and uniform to maintain.

`go-yze` owns the contract between the analyzers and the [`stickler`](https://github.com/gomatic/stickler) runner:

- the normalized `Diagnostic` / `Fix` / `TextEdit` schema (the lean shape every tool's output is normalized into),
- analyzer `Registration` (identity + the `group`/`category` taxonomy),
- the `Reporter` that turns a `go/analysis` finding into a `Diagnostic` carrying a stable rule id, help URL, and an optional mechanical fix,
- `Main`, the one-line `singlechecker` entry point each analyzer's `cmd` calls,
- `ApplyFixes`, the single shared engine that applies `TextEdit`s and re-`gofmt`s (powering `yze --fix` and `stickler --fix`),
- the `analysistest` harness every analyzer's tests reuse.

It is a pure library: CLI-agnostic and dependency-light (`go-error` for sentinel errors, `x/tools` for `go/analysis`, `testify` for tests).
