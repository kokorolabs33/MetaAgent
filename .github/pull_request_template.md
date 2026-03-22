## Summary

<!-- 2-5 bullets: What changed and why -->

-

## Change Type

- [ ] Bug fix
- [ ] New feature
- [ ] Refactor
- [ ] Docs
- [ ] Chore (deps, CI, config)

## Scope

- [ ] Backend (Go)
- [ ] Frontend (Web)
- [ ] Both
- [ ] Infrastructure (CI, Docker, config)

## Linked Issue

<!-- Closes #N or Related #N -->

## Type Contract

<!-- If you changed Go models or TypeScript interfaces, confirm both sides are updated -->

- [ ] N/A — no model/type changes
- [ ] Go struct updated with correct `json:"..."` tags
- [ ] TypeScript interface in `web/lib/types.ts` mirrors Go changes
- [ ] SQL migration added/updated if new columns

## Security Impact

- [ ] No security impact
- [ ] New/changed API endpoints
- [ ] Database schema changes
- [ ] Authentication/authorization changes
- [ ] New external service calls

<!-- If any box above is checked (except "No security impact"), explain: -->

## Verification

<!-- What you tested and how -->

- [ ] `make check` passes
- [ ] Manually tested locally
- [ ] Added/updated tests

**Steps to verify:**

1.

## Bug Fix Evidence

<!-- Required for bug fixes. Delete this section for features/refactors. -->

- **Symptom:** <!-- What was broken? Logs, screenshots, repro steps -->
- **Root cause:** <!-- Which file/function, why it failed -->
- **Fix:** <!-- What the fix does -->
- **Regression test:** <!-- Test added? If not, why? -->

## Rollback

<!-- How to revert if something goes wrong -->

- [ ] Safe to revert commit (no migration)
- [ ] Requires migration rollback — steps: <!-- describe -->
