# openclaw-image-build-ram — building openclaw needs ~4 GB free RAM; runtime needs ~200 MB

**Filed:** 2026-06-15
**Status:** Open — option 1 ships in v1.6.19; options 2–5 staged for follow-up patches.
**Severity:** High — clients on small VPS / EC2 boxes (t3.micro, $5 droplets) hit OOM-kill during `claws image bootstrap` and have no path forward without buying a bigger box.

---

## The problem

`claws image bootstrap` clones `github.com/openclaw/openclaw` and runs
`docker build`. The build is heavy:

- Node bundler with full dev dependencies (TypeScript + esbuild +
  webpack-class loaders)
- Multiple compile stages
- Peak memory hits ~3–4 GB during the bundle

The **runtime** is light — once `openclaw:local` is built, agents
need ~200 MB each. A box that can comfortably run 5 agents
(~1 GB total) can't build the image once.

Symptoms:

```
$ claws image bootstrap --yes
==> Step 3 — source build
  $ docker build -t openclaw:local /tmp/claws-openclaw-build
  Step 8/12 : RUN npm run build
  Killed
ERROR: docker build failed: exit status 137
```

Exit 137 = SIGKILL (usually OOM-kill). The user has no signal to
correlate this with "your box doesn't have enough RAM."

---

## The five options

Ordered "ship today" → "right architectural answer."

### Option 1 — Temporary swapfile during build *(ships in v1.6.19)*

`claws image bootstrap --add-swap[=SIZE]` creates a swapfile before
`docker build`, runs the build, removes the swapfile (success OR
failure path; signal handler covers Ctrl-C).

```
$ claws image bootstrap --yes --add-swap
==> Step 3 — source build
    Adding 8 GB temporary swapfile at /tmp/claws-bootstrap.swap
    $ sudo fallocate -l 8589934592 /tmp/claws-bootstrap.swap
    $ sudo chmod 600 /tmp/claws-bootstrap.swap
    $ sudo mkswap /tmp/claws-bootstrap.swap
    $ sudo swapon /tmp/claws-bootstrap.swap
    ✓ swap active

  (docker build runs, slower but completes)

    Cleaning up swap:
    $ sudo swapoff /tmp/claws-bootstrap.swap
    $ sudo rm /tmp/claws-bootstrap.swap
    ✓ swap removed
```

**Pros:** ~150 LOC. Works on a $5 VPS today. Operator already needs
sudo for `docker build` group membership, so the sudo escalation
isn't new. Linux-only is fine because docker hosts are Linux-only
(macOS Docker Desktop manages its own RAM).

**Cons:** Slow as hell — disk-bound for the swap-heavy bundle stage.
Kills SSD endurance if the box is rebuilt frequently (one rebuild
≈ 1 GB written; 100 rebuilds = noticeable wear). The right
recommendation: do it **once** per host, not on every release.

**Memory check (part of this option):**
- Before docker build, read `/proc/meminfo` for `MemAvailable`.
- If < 4 GB AND `--add-swap` not passed → warn loudly with the
  size + the four other options (defer to this ticket for the
  full menu).
- If < 4 GB AND `--add-swap` passed → proceed with swap.
- If ≥ 4 GB → proceed normally; mention the runtime requirement in
  `claws doctor` for honesty.

### Option 2 — Pre-built image in the release tarball *(planned)*

claws's release tarball includes `openclaw.tar` (output of
`docker save openclaw:local`). `claws image bootstrap` does
`docker load < openclaw.tar` instead of `docker build`.

**Pros:** Zero RAM for setup. ~10 second install instead of 5
minute build. Works offline (useful for air-gapped corporate
hosts). Operators don't need to think about RAM at all.

**Cons:** Release tarballs grow from ~10 MB to ~1 GB. Bandwidth
heavy on every install. Probably want to host `openclaw.tar` as a
separate optional download alongside the binary, not bundled into
the main tarball.

**Implementation sketch:**
- Release pipeline saves the image: `docker save openclaw:local | gzip > openclaw-vX.Y.Z.tar.gz`
- claws's install.sh learns `--with-image` flag that downloads + loads it
- `claws image bootstrap` learns `--load-from=<file>` flag

**Defer because:** Bandwidth cost; need to decide whether this is
opt-in (`install.sh --with-image`) or default.

### Option 3 — Publish openclaw to GHCR / Docker Hub *(planned — the right answer)*

`claws image bootstrap` becomes `docker pull ghcr.io/ab0t-com/openclaw:latest`.

**Pros:**
- Zero RAM for setup
- Fast install (cached on Docker's CDN)
- Versioned via tags (`openclaw:v2026.6.1`)
- Standard pattern; users already understand `docker pull`
- Heavily cached after first pull

**Cons:**
- Requires an openclaw CI pipeline (GitHub Actions on the openclaw
  repo) that builds + pushes to `ghcr.io/ab0t-com/openclaw` on every
  release. This is **separate work in a different repo** (openclaw,
  not claws).
- First pull requires internet
- Image hosting cost (GHCR is free for public repos; Docker Hub has
  pull-rate limits unless paid)

**Implementation sketch:**
- openclaw repo gets `.github/workflows/release.yml` that does
  `docker buildx + docker push ghcr.io/ab0t-com/openclaw:$tag`
- claws's `image bootstrap` resolution order becomes:
  1. `--source=` flag / `OPENCLAW_IMAGE_SOURCE` env (existing)
  2. `ghcr.io/ab0t-com/openclaw:latest` (new default)
  3. Source build fallback (existing)
- The "pull from GHCR" path becomes the silent happy path; the
  source-build path becomes the rare offline / hacking fallback.

**This is the strict opinionated answer for an OSS distribution
story.** Pre-built images on a public registry is what every other
project does. The work is real but not in claws — coordinate with
the openclaw maintainers.

### Option 4 — Remote builder via `docker buildx` *(niche; probably skip)*

For ops who have a beefy build host AND want to run claws on small
boxes. `claws image bootstrap --build-on=ssh://buildhost.local`.

`docker buildx create --driver-opt host="ssh://..."` runs the build
on the remote host, ships the image to the local docker daemon.

**Pros:** Build farm pattern. Useful for shops with a build VM and
many small runtime boxes.

**Cons:** Niche. Requires a second SSH-accessible Docker host. Most
operators won't have this; the documentation cost outweighs the
implementation cost for the small number of users who'd benefit.

**Recommendation:** Document the pattern, don't build a flag for it.
Users with this need can do it manually:

```bash
docker buildx create --name claws-builder --driver-opt host="ssh://buildhost"
docker buildx use claws-builder
cd /tmp/claws-openclaw-build && docker buildx build -t openclaw:local --load .
```

Add this to the docs page for option 1's "alternatives if you'd
rather not swap."

### Option 5 — Document the requirement honestly + fail-fast *(ships partially in v1.6.19)*

Whatever else we do, make the RAM requirement visible:

- **`claws doctor`** checks `MemAvailable` and warns if < 4 GB
  before `image bootstrap` is run.
- **`claws image bootstrap`** runs a RAM check before `docker
  build`, prints the recommendation, lists the four other options
  in this ticket.
- **README Prerequisites** adds: *"Building openclaw needs ~4 GB
  free RAM; runtime needs ~200 MB. If you don't have 4 GB, use
  `claws image bootstrap --add-swap`, or build on a bigger box and
  copy the image."*
- **`scripts/prereqs/check.sh`** could grow a `memory` check, but
  that's more of an "is claws happy" thing than a "can claws install"
  thing — defer.

Cheap, honest, prevents the user spending 20 minutes wondering why
their `docker build` got OOM-killed.

---

## What ships in v1.6.19 (option 1 + part of option 5)

- `claws image bootstrap --add-swap[=SIZE]` — temporary swapfile
  around the docker build, with full sudo-aware lifecycle and
  signal-handler cleanup.
- `claws image bootstrap` — reads `/proc/meminfo` for `MemAvailable`
  before `docker build`; if < 4 GB and `--add-swap` not passed,
  prints a clear warning + the four other options + asks for
  confirmation to proceed.
- `claws doctor` — surface available memory in its diagnostics so
  the gap between "you have N GB" and "you need ~4 GB to build" is
  visible.
- README — Prerequisites section adds the RAM requirement and the
  `--add-swap` workaround.

**Out of v1.6.19** (planned for follow-up patches):

- Option 2 — pre-built image in release tarball
- Option 3 — GHCR pipeline (needs openclaw repo coordination)
- Option 4 — remote builder (probably document-only, never code)

---

## Open questions

1. **Default swap size.** 8 GB is generous; 4 GB might be enough.
   Pick 8 GB initially, let operators override via `--add-swap=4g`
   for tighter disks. Bottom of `/tmp` should comfortably hold 8 GB
   on any host running claws.

2. **Should `--add-swap` be sticky?** I.e. once created, leave it
   in place? **No.** That would change the host's permanent swap
   config without consent. The swapfile is created+removed within
   the bootstrap invocation. Operators who want persistent swap can
   add it to `/etc/fstab` themselves.

3. **Should the RAM check default to refusing the build?** No. The
   v1.6.19 implementation **warns but proceeds** unless `--require-ram`
   is passed. Forcing operators to opt out of a build is heavier
   than warning them.

4. **macOS support.** `--add-swap` is a no-op on macOS with a clear
   warning ("on macOS, configure RAM in Docker Desktop Settings →
   Resources"). Docker Desktop manages its own RAM allocation.

5. **`/tmp` size.** If `/tmp` is on a small partition (some
   container hosts have tmpfs /tmp), the swapfile creation will
   fail. Fall back to `~/.cache/claws/bootstrap.swap` (auto-detect
   free space on each candidate).

---

## Acceptance criteria

For v1.6.19 (option 1 + partial option 5):

- A box with 2 GB RAM can run `claws image bootstrap --add-swap`
  and end up with `openclaw:local` present, swapfile removed,
  daemon state unchanged.
- A box with 8 GB RAM running `claws image bootstrap` (no flag)
  builds normally, no swap added, no warning printed.
- A box with 2 GB RAM running `claws image bootstrap` (no
  `--add-swap` flag) gets a clear warning before the build attempt:
  current RAM, recommended RAM, the four other options, then
  proceeds if confirmed.
- Ctrl-C during the docker build cleanly removes the swapfile via
  the signal handler.
- `--add-swap` on macOS is a no-op with a clear "configure RAM in
  Docker Desktop" message.

---

## References

- `cmd/claws/image_bootstrap.go` — current bootstrap logic
- `cmd/claws/doctor.go` — where the memory check belongs
- Docker's official position on building on small boxes:
  https://docs.docker.com/build/ (relevant: BuildKit caches, slim
  base images, multi-stage)
- swapfile-at-runtime pattern is standard on Linode / DO docs for
  small VPS users.
