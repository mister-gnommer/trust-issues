# AGENTS.md — trust-issues

Shared instructions for OpenCode and Claude Code. Read before making changes.

---

## 1. I'm learning Go

I'm a mid-level TypeScript backend developer. I'm using this project to learn Go natively.

**Do not write TypeScript in Go.** Push back if I propose patterns that are idiomatic in TS but not in Go. Suggest the Go way instead.

Common TS→Go traps to flag:

| TS habit | Go alternative |
|---|---|
| Classes for everything | Structs + functions with receivers; package-level functions |
| Inheritance / extends | Composition via struct embedding |
| Promises / async/await | Goroutines + channels, `errgroup`, explicit error returns |
| `try/catch` | Explicit `if err != nil` |
| Generics everywhere | Use generics sparingly; concrete types are preferred |
| Decorators / higher-order wrappers | Middleware as functions, explicit composition |
| `Array.map` / `.filter` | `for` loops; slices; Go 1.23+ range-over-func iterators if needed |
| ORM / query builder | Plain SQL (already in use here via `database/sql`) |
| Union types / discriminated unions | Interfaces, `switch` on concrete type, or `iota` enums |
| Optional chaining `?.` | `if x != nil { x.Field }` |
| `enum` keyword | `const` block + `iota` (already the project convention) |

---

## 2. Go tooling

After making code changes, run `go vet ./...` and `go test ./...`. If either fails, fix before proceeding.

Build: `go build ./cmd/trust-issues/`

---

## 3. Code attribution

- **New files:** Add `// 🤖 AI-generated` at the top. I'll remove it on review.
- **Autonomous changes** ("just do it" mode): wrap added/modified code with `// 🤖 AI-start` / `// 🤖 AI-end` fences. Fence only your changes, not surrounding code.
- **Small trivial changes** (typos, single-line tweaks): skip fences even in autonomous mode.
- **Directed changes** (I tell you exactly what and how to change): no comment needed.

---

## 4. Project conventions

- Zero-value initialization is fine — `var x Type` over `x := new(Type)` unless you need the pointer.
- Errors propagate via `fmt.Errorf("context: %w", err)`. Never discard errors silently.
- SQLite queries use `?` placeholders (driver convention, not `$1`).
- Logging uses `log/slog` with structured key-value pairs.
- Config is TOML, parsed by `BurntSushi/toml`.
- Interfaces are defined in the consumer package (e.g. `playback.Source` not `spotify.Source`).
- Tests use `:memory:` SQLite databases. Follow existing test style.
