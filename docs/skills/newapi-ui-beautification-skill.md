---
name: newapi-ui-beautification-skill
description: End-to-end workflow for KIE-style UI beautification, image build, and safe Docker deployment without changing backend API behavior.
version: 1.0.0
---

![newapi_ui_001 preview](https://raw.githubusercontent.com/uskyu/new-api/newapi_ui_001/example_PNG/001.png)

# New API UI Beautification Skill

## Mission

Deliver and deploy a UI beautification release for `new-api` while keeping backend behavior stable.

## Non-negotiable rules

1. Do not change backend API behavior.
2. Do not change logo API/reference behavior.
3. Do not refactor icon reference/import/mapping mechanisms.
4. Keep i18n complete for newly added homepage text.
5. Prefer UI-only changes in `web/src/**`.
6. For console optimization, prioritize user-visible pages; admin-only pages are optional unless requested.

## Scope template

Use this scope when repeating the same release style:

- Home page visual redesign (KIE-like style).
- Top header/navigation redesign and route-aware theme sync.
- Console layout continuity (top bar + left sidebar + content seams).
- Console user pages beautification (token, log, midjourney, task, topup, personal, playground).
- Pricing/model plaza beautification, with matching header palette.

## Required implementation checklist

1. Keep structure and interfaces unchanged.
2. Apply styles via CSS classes and page-level wrappers.
3. Preserve existing data flow and route logic.
4. Keep dark mode and mobile layout usable.
5. Run frontend build verification after changes.

## Build verification

Run in `web/`:

```bash
bun install
bun run build
```

Treat warnings as non-blocking unless build fails.

## Release workflow (fork-based)

1. Commit UI changes on branch `ui-beautification-version`.
2. Push branch to fork.
3. Build container image in GitHub Actions (avoid compiling on low-resource server).

Reference workflow file:

- `.github/workflows/ui-beautification-image.yml`

Expected pushed image tags:

- `ghcr.io/<owner>/new-api:ui-beautification`
- `ghcr.io/<owner>/new-api:ui-beautification-<sha>`

## Server deployment (keep existing docker-compose)

Assume compose service uses `calciumion/new-api:latest`.

```bash
docker pull ghcr.io/<owner>/new-api:ui-beautification
docker tag ghcr.io/<owner>/new-api:ui-beautification calciumion/new-api:latest

cd <compose_project_dir>
docker compose -p <compose_project_name> up -d --force-recreate --no-deps new-api
docker compose -p <compose_project_name> logs -f --tail=100 new-api
```

Why this works:

- `docker tag` retargets compose image name to the newly pulled UI image.
- `--force-recreate --no-deps` replaces only `new-api` container, leaving `postgres/redis` intact.

## Safety and rollback

Image backup before replacement:

```bash
OLD_IMG=$(docker inspect -f '{{.Image}}' <running_new_api_container>)
docker image tag "$OLD_IMG" calciumion/new-api:before-ui-$(date +%Y%m%d-%H%M%S)
```

Rollback:

```bash
docker tag calciumion/new-api:before-ui-<timestamp> calciumion/new-api:latest
docker compose -p <compose_project_name> up -d --force-recreate --no-deps new-api
```

Optional DB backup:

```bash
docker exec -t postgres pg_dump -U root -d new-api > /root/newapi_backup_$(date +%F_%H%M).sql
```

## Low-resource server guidance

If server panel becomes unresponsive during local `docker build`, avoid server-side build and use CI-built images only.

Emergency controls:

```bash
pkill -f "docker build"
pkill -f "buildkit"
pkill -f "go build"
```

Then recover panel/service and continue with image pull/tag/recreate flow.

## Definition of done

1. UI pages match requested theme direction.
2. No backend/API contract changes introduced.
3. `bun run build` succeeds.
4. Container redeploy succeeds with existing compose config.
5. Service logs show healthy startup.
