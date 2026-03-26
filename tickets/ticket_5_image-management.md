# Ticket 5: Image Management — Build, Pull, Pin, Upgrade

**Priority:** P2 — Medium
**Created:** 2026-03-25
**Status:** Done

## Problem

Currently:
- Image is always `openclaw:local` (locally built, mutable tag)
- No `docker pull` path — can't pull from a registry
- No version pinning — rebuilding the image changes what all instances run
- No `clawctl upgrade` command
- No way to run different instances on different image versions
- Admin cannot restrict which images are allowed (ticket 2 prerequisite)

## Design

### Image Sources

```bash
clawctl image build                           # build from local openclaw repo
clawctl image build --tag=v2026.3.25          # build with specific tag
clawctl image pull openclaw:latest            # pull from registry
clawctl image pull ghcr.io/openclaw/openclaw:v2026.3.25  # pull specific version
clawctl image list                            # list local images
clawctl image pin <instance> <image:tag>      # pin instance to specific image
```

### Per-Instance Image Override

Stored in `instance.env`:
```
OPENCLAW_IMAGE=openclaw:v2026.3.25
```

Compose template already supports this via `${OPENCLAW_IMAGE:-openclaw:local}`.

### Upgrade Flow

```bash
clawctl upgrade <instance>                    # pull latest, restart
clawctl upgrade <instance> --image=openclaw:v2026.4.1  # upgrade to specific version
clawctl upgrade --all                         # upgrade all instances
clawctl upgrade --all --rolling               # rolling restart (one at a time)
```

Upgrade steps:
1. Pull new image (or verify it exists)
2. Update `OPENCLAW_IMAGE` in `instance.env`
3. Stop instance
4. Start instance with new image
5. Wait for health check
6. If health fails, rollback to previous image

### Policy Integration (Ticket 2)

```json5
{
  "allowedImages": ["openclaw:local", "openclaw:v*", "ghcr.io/openclaw/*"],
  "requireImageDigest": false
}
```

## Implementation

1. `image.go` — image management commands
2. Update `cmdCreate` to accept `--image=` flag
3. `cmdUpgrade` with health-check rollback
4. Integration with policy for image restriction

## Testing
- Integration test: create with custom image tag
- Integration test: upgrade with health-check pass
- Integration test: upgrade with health-check fail → rollback
- Integration test: policy blocks disallowed image
