# Check-in + Risk Control Handoff

Branch: `HelpSelf-latest`
Status: implemented and verified locally, pending user acceptance.

## Completed since last GHCR image

- Check-in self-hosted captcha (math/digit) using `base64Captcha`, in-memory challenge store, 5-minute TTL, single-use, 5 attempts, no Redis dependency.
- Check-in active tier rewards based on yesterday usage (`request_count` or `quota_consumed`), tiers use threshold + min/max reward range, highest tier wins, no probability field.
- Admin check-in settings for classic and default templates: captcha switch/type, bonus switch/metric, tier table with units (calls/quota).
- User check-in cards for classic and default templates: captcha modal and active-tier reward hint.
- Classic template IP recording switch bug fix (missing `onChange` made saved `false` revert to `true`).
- Risk control multi-search: search type select for IP / username+nickname / user ID, backend `search_type` parameter, classic and default templates, i18n added.
- i18n updated for classic (8 files) and default (6 files).
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

1. Admin check-in settings: after selecting captcha type / bonus metric, saving and refreshing collapses/hides those fields again. The switch remains enabled but the conditional fields do not stay expanded/visible. Reproduce in both classic and default templates, then fix state/expansion handling.
2. Mobile UI: verify and finish mobile layouts for the check-in captcha modal, tier table in admin settings, and risk-control search controls. Current desktop layouts work; mobile needs a final pass.
3. Confirm risk-control username search covers `display_name` as nickname (backend already queries `username LIKE OR display_name LIKE`; keep this behavior).

## Not committed yet

Nothing beyond this branch state. Do not trigger GHCR until the open issues are resolved and the user approves.