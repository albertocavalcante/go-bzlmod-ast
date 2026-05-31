# tools/

Dev/CI tool acquisition scripts that aren't covered by the Go module
ecosystem. Keep this directory small and outcome-oriented; if a tool
can be pinned via `tools.go.mod`, prefer that over a custom script.

## Inventory

- **[install-just.sh](install-just.sh)** — hermetic install of the
  [`just`](https://github.com/casey/just) recipe runner. Pinned by
  version + SHA-256 per platform, idempotent, atomic.

For Go-based tooling (golangci-lint, gotestsum, staticcheck-bundled)
see [`tools.go.mod`](../tools.go.mod) at repo root — pinned via
`tools.go.sum` and verified against `sum.golang.org` by the standard
`go build -modfile=tools.go.mod ...` invocation.

## Threat model

CI for this library is library-only: no secrets, no deploy targets,
no released binaries. Realistic harms from a CI compromise:

- **Source exfiltration** — low (source is already public).
- **Tagged release poisoning** — medium (downstream consumers
  refresh against tags; a poisoned tag would propagate).
- **CI runner persistence** — medium (a compromised binary stays
  resident across runs until the runner is rebuilt).

The bar is "make supply-chain shifts visible at code-review time,"
not "defend against nation-states." Concretely, that means:

1. Pin every external binary by version AND content hash.
2. Pin every GitHub Action by commit SHA, not tag.
3. Use the Go module ecosystem's `sum.golang.org` checksumDB for
   Go-based tooling — it's strong by default.
4. Verify hashes against upstream releases before committing a bump.
5. Quarterly review of all pins; immediate bump on a known CVE.

## Bumping `install-just.sh`

When [`just`](https://github.com/casey/just/releases) releases a new
version we want:

1. Pick the target version from the release tag.
2. Download all four supported tarballs locally:
   ```bash
   V=1.52.0   # the new version
   for triple in \
       x86_64-unknown-linux-musl \
       aarch64-unknown-linux-musl \
       x86_64-apple-darwin \
       aarch64-apple-darwin
   do
       curl -sSL --proto '=https' --tlsv1.2 --fail \
           -O "https://github.com/casey/just/releases/download/${V}/just-${V}-${triple}.tar.gz"
   done
   ```
3. Compute SHA-256 for each:
   ```bash
   shasum -a 256 just-${V}-*.tar.gz
   ```
4. Update `JUST_VERSION` and the four entries in `JUST_SHA256` in
   `install-just.sh` atomically.
5. Cross-reference against the GitHub release's
   [checksums.txt](https://github.com/casey/just/releases) if
   published — the values must match what `shasum` produced. Do NOT
   lift hashes from third-party install scripts.
6. Verify locally: `./tools/install-just.sh /tmp/just-bump-test`
   should download, verify, and install cleanly. Repeat with
   `INSTALL_DIR=/tmp/just-bump-test ./tools/install-just.sh` —
   second call must skip (idempotence).
7. Commit; push to a branch; CI verifies it installs in Forgejo's
   Linux x86_64 runner before merge.

## What's deliberately not in this directory

- **GitHub Action versions** — those are pinned by commit SHA inside
  `.forgejo/workflows/ci.yml` and `.github/workflows/ci.yml`. They
  follow the same threat-model but live with the workflows they
  configure rather than here.
- **Go tools** — pinned via `tools.go.mod` at repo root; checksum
  verification handled by the Go ecosystem (`tools.go.sum` +
  `sum.golang.org`). No script needed.
- **Runner image** — out of scope; the Ubuntu runner is provided by
  the Forgejo Actions admin (currently Alberto). Hardening that is a
  separate infra concern.

## Cadence

- **Quarterly review** — run `just audit-pins`, check each pinned
  version against upstream's current release, bump if there's a
  reason to.
- **On CVE notification** — bump the affected tool immediately.
  GitHub Security Advisories cover `actions/checkout`,
  `actions/setup-go`, and most Go modules; `just` doesn't currently
  publish advisories so we rely on its release-notes for security
  context.

