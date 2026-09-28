# CodeGraph Visualization

These SVGs are a compact visual pass over the current repository structure.

- [Architecture map](architecture.svg): high-level module topology.
- [Runtime turn flow](runtime-turn-flow.svg): one user turn from entry surface through provider/tool execution and event consumers.

Source inputs were CodeGraph commands run from the repository root:

- `codegraph init .`
- `codegraph status .`
- `codegraph files`
- `codegraph query "Run"`
- `codegraph query "Build"`
- `codegraph query "Submit"`
- `codegraph explore "internal boot Build config provider controller tools plugin skills memory setup"`
- `codegraph explore "internal agent Agent Run provider Stream tool Registry tool calls compact session sink events"`

The diagrams intentionally collapse many package-level edges into a small number of runtime roles so the result remains readable.
