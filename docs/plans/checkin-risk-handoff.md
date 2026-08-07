# Check-in + Risk Control Handoff

Branch: `HelpSelf-latest`
Status: user accepted (verified on live preview), GHCR image build triggered.

## Completed since last GHCR image

- Check-in self-hosted captcha (math/digit) using `base64Captcha`, in-memory challenge store, 5-minute TTL, single-use, 5 attempts, no Redis dependency.
- Check-in active tier rewards based on yesterday usage (`request_count` or `quota_consumed`), tiers use threshold + min/max reward range, highest tier wins, no probability field.
- Admin check-in settings for classic and default templates: captcha switch/type, bonus switch/metric, tier table with units (calls/quota).
- User check-in cards for classic and default templates: captcha modal and active-tier reward hint.
- Classic template IP recording switch bug fix (missing `onChange` made saved `false` revert to `true`).
- Risk control multi-search: search type select for IP / username+nickname / user ID, backend `search_type` parameter, classic and default templates, i18n added.
- i18n updated for classic (8 files) and default (6 files).
- Admin tier table mobile UX: each tier input now shows its unit on small screens (`Threshold (calls) / Threshold (tokens) / Minimum Reward (tokens) / Maximum Reward (tokens)` labels under `md:` breakpoint), so operators can tell calls vs token inputs on phones.
- `go.mod` dependency cleanup: `base64Captcha` moved to direct dependency.

## Verification

- `go test ./model ./controller ./service` passed for the touched areas; existing unrelated service affinity-cache tests still fail.
- `go build ./...` passed.
- `go vet` passed for model/controller/service/router/dto.
- `bun run build` passed for both classic and default frontends.
- Default template `tsc` typecheck passed.

## Local demo data

Seed script: `scripts/seed-risk-demo.go`

```bash
go run scripts/seed-risk-demo.go
```

It idempotently removes previous `risk-demo-%` users/records and seeds 45 users, 90 risk records (15 shared-IP token groups plus unique login IPs), plus inviter relationships for the invitation tab.

## Open issues for next developer

1. **FIXED** — Admin check-in settings: after selecting captcha type / bonus metric, saving and refreshing collapses/hides those fields again. The switch remains enabled but the conditional fields do not stay expanded/visible.
   - **Root cause (default template)**: react-hook-form unregisters (and drops the default value of) fields that unmount with conditional rendering. When a parent switch is toggled off and back on, the child fields (`captchaKind`, `bonusMetric`) mount again with `undefined`, which makes the conditional sections stay collapsed and the select values empty.
     - Fix in `web/default/.../checkin-settings-section.tsx`: watch values fall back to `defaultValues` (`form.watch('captchaEnabled') ?? defaultValues.captchaEnabled`, same for enabled/bonusEnabled/bonusMetric) and `FormField defaultValue={defaultValues.captchaKind/bonusMetric}` so re-mounted fields restore their value.
   - **Root cause (classic template)**: `OperationSetting.getOptions` replaced state with `newInputs = {}` so option keys absent from the backend (e.g. never-saved check-in keys) were dropped from state and the boolean type guard (`typeof inputs[key] === 'boolean'`) degraded, leaving `"true"/"false"` strings that a plain truthiness check handles but Semi fields misread. `SettingsCheckin` also passed the invalid `values` prop (Semi uses `initValues`) and did not normalize boolean strings.
     - Fix in `web/classic/.../OperationSetting.jsx`: `newInputs = { ...inputs }` keeps existing keys and re-uses `newInputs` for the boolean conversion.
     - Fix in `web/classic/.../SettingsCheckin.jsx`: `Form initValues={inputs}` (valid Semi prop) + normalize `"true"/"false"` strings back to booleans when rebuilding `currentInputs`.
   - **E2E verified** on both templates via Playwright: toggling captcha/bonus off→on restores the conditional fields with values preserved, and after save + page reload the conditional fields stay expanded.
2. **FIXED** — Mobile UI: the admin tier table inputs showed no unit on phones, making it unclear whether a field is a call count or a token amount. Each tier input now renders a small unit label below `md:` (threshold label switches with `bonus_metric`), in both `web/classic/.../SettingsCheckin.jsx` and `web/default/.../checkin-settings-section.tsx`; the delete button wraps to its own row on mobile (`col-span-2 md:col-span-1`). Captcha modal and risk-control search controls already had responsive layouts. **Verified** on both templates via Playwright at 375px viewport: `Threshold (calls)`, `Minimum Reward (tokens)`, `Maximum Reward (tokens)` labels visible.
3. **Verified** — Risk-control username search covers `display_name` as nickname: backend queries `username LIKE OR display_name LIKE` (`model/risk_control.go:154`), confirmed against seeded data.

## Demo database (ready for manual testing)

Seeded SQLite DB `one-api.db` (backend compiled from this branch, both frontends built and embedded):

- Admin: **root / 12345678** (8+ chars so the default template password validation accepts it).
- Demo users: **risk-demo-001 .. risk-demo-045 / risk-demo-pass** (bcrypt-hashed), with quota, inviter relationships (invitation tab), shared-IP token groups + unique login IPs (risk control), and per-user yesterday usage logs across all active-tier thresholds.
- Check-in is enabled with math captcha + active tier rewards; tiers: `>=20` calls → 5000–20000, `>=100` calls → 20000–100000. Users `risk-demo-001..020` already checked in today.
- Seed script `scripts/seed-risk-demo.go` is idempotent (resets root, clears previous demo users/logs/checkins/risk records) and now runs in batches for the logs insert (SQLite variable limit).

## Committed

Committed to `HelpSelf-latest` with `[skip ci]` (no GHCR/CI triggered). Local `one-api.db` with seeded demo data is available in the workspace for manual testing; the backend binary at `/tmp/new-api-server` embeds both freshly built frontends.

## Released

- User acceptance testing on the live preview passed (check-in captcha, active-tier rewards, admin settings persistence, risk-control multi-search, mobile layouts).
- This commit (no `[skip ci]` marker) pushes to `HelpSelf-latest`, which triggers the GHCR image workflow (`ghcr-image.yml`, tags: `helpself-latest` + `helpself-latest-<short-sha>`).