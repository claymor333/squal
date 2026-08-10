# config — profiles + AI endpoint

JSON config, mode 0600, env overrides. Never commit it.

## Owns

| File | Role |
|------|------|
| `config.go` | `Profile` (connections), `AI` (base_url/model/key), `Config`, `Load`/`Save`/`Path`, `AddProfile`/`RemoveProfile`, `APIKey` |

## Invariants (do not break)

- **Credentials never enter source, history, or commit messages.** File is 0600.
- **Env overrides file**: `SQUAL_OPENAI_API_KEY` wins over the file key;
  `SQUAL_CONFIG` overrides the default path.
- **`Profile.Database` is optional** — it's a default-schema hint only. The TUI
  shows all databases the credentials can see (`SHOW DATABASES`); the database
  selected in the tree (`connData.currentDB`) is the primary source for
  unqualified editor queries via `db.FetchOn`.
- **`AddProfile` upserts by name; `RemoveProfile` is an in-place filter.**
- No secrets in shell history when launching.

## Change here

- New connection field → extend `Profile` (+ DSN builder in `db/conn.go`).
- New AI option → extend `AI`.

## Tests

Config is exercised through the app; the tui `connect_test.go` covers
`AddProfile`/`RemoveProfile`/`buildProfile`.

## Doc check

Did this change touch an invariant above? If yes, update it here.
