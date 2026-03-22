# PR Maintainer Skill

Use this skill when triaging, reviewing, or landing pull requests.

## PR Triage Flow

When a PR arrives, follow this decision path:

```
1. Does the PR have a clear description?
   NO → Request description using PR template
   YES → continue

2. Does it pass CI?
   NO → Comment with failing checks, request fix
   YES → continue

3. Is the type contract maintained?
   (Go models ↔ TypeScript types ↔ SQL schema)
   NO → Request updates
   YES → continue

4. Does it follow commit conventions?
   NO → Request squash/reword
   YES → continue

5. Review code using $code-review skill
```

## Landing a PR

Before merging any PR:

1. **CI green** — all checks pass
2. **Reviewed** — at least one approval
3. **No unresolved conversations**
4. **Rebase on main** — no merge commits
5. **Commit message** — follows conventional format: `feat|fix|refactor|docs|test|chore(scope): description`

### Merge strategy

- **Squash merge** for single-purpose PRs
- **Rebase merge** for well-structured multi-commit PRs where each commit is meaningful
- **Never** create merge commits on main

## Commit Convention

Format: `type(scope): description`

Types: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`
Scopes: `backend`, `web`, `ci`, `docker`, or omit for cross-cutting

Examples:
- `feat(backend): add cost tracking endpoint`
- `fix(web): SSE reconnection race condition`
- `chore(ci): add frontend type-check step`
- `refactor: move Go code to project root`

## PR Labels

| Label | Meaning |
|-------|---------|
| `bug` | Bug fix |
| `enhancement` | New feature |
| `refactor` | Code improvement without behavior change |
| `docs` | Documentation only |
| `breaking` | Contains breaking changes |
| `needs-review` | Awaiting review |
| `needs-changes` | Review feedback pending |

## Safety Rules

- Ask confirmation before closing >3 PRs at once
- Never force-push to main
- Never push merge commits to main — rebase first
- Do not modify CI workflow files without explicit approval
