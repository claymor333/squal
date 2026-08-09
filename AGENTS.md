# Squal — Project Conventions

## Superpowers

At the start of every session, load the `superpowers-using-superpowers` skill via the `skill` tool and follow it. It establishes how to find and use the superpowers skills in this repository.

## Conventions

- Follow existing code conventions in this application. When creating or editing a file, check sibling files for structure, approach, and naming.
- Go: `gofmt` clean, `go vet` clean. No comments unless they carry meaning beyond the code. Standard library first; reach for a dependency only when stdlib is genuinely worse.
- Use descriptive names. `isQueryRunning`, not `q`.
- Prefer columnar data for anything that gets sorted/filtered on the client (see SPEC §4).

## Commit Messages

Use conventional commits: `<type>(<scope>): <body>`

| Part | Meaning | Examples |
|------|---------|---------|
| `type` | `feat`, `fix`, `test`, `docs`, `refactor`, `chore` | `feat`, `fix` |
| `scope` | Module/namespace | `tui`, `db`, `ai`, `state`, `config` |
| `body` | Present-tense description | `stream query results into columnar grid` |

Example: `feat(tui): render virtualized result grid with client-side sort`

PR titles follow the same format, derived from the most substantive commit in the set.

## Destructive Git Operations — NEVER DO THIS

Never force-push, delete branches, rewrite history, hard-reset, or `git clean`. If you need to undo, create a new commit or PR that reverses the change.

## Environment

- Go toolchain lives at `~/go-sdk/go/bin` (export `PATH="$HOME/go-sdk/go/bin:$PATH"` if `go` isn't on PATH).
- Local MariaDB for testing: `127.0.0.1:3306` user `webstack` / pass `webstack` (docker `mariadb-master`), scratch schema `squal_smoke`.
- Smoke verification: `go run ./cmd/smoke` — requires the scratch DB grant (see history; already granted).

## Verification Scripts

Do not create verification scripts or tinker when tests cover that functionality. `cmd/smoke` is the exception — it's the integration check against a real MariaDB.

## Documentation Files

Only create documentation files if explicitly requested.
