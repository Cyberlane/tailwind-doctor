# Releasing

Tailwind Doctor releases are built from a signed Semantic Version tag. The tag
workflow reruns every test, verifies the tag through GitHub, builds the release
from that exact commit, publishes npm packages with OIDC trusted publishing,
attests the archives, creates the GitHub release, and deploys the documentation
site with that exact release version.

## Distribution

Two user-facing packages ship in lockstep:

- `tw-doctor` exposes both `tw-doctor` and `tailwind-doctor` commands and selects
  the current platform binary through `optionalDependencies`.
- `tailwind-doctor` is a thin alias package so `npx tailwind-doctor` resolves by
  package name.

The platform packages are scoped so an unclaimed target name cannot be
registered by another publisher:

- `@tw-doctor/darwin-arm64`
- `@tw-doctor/darwin-x64`
- `@tw-doctor/linux-arm64`
- `@tw-doctor/linux-x64`
- `@tw-doctor/win32-x64`

Every platform package contains only its statically linked Go binary, README,
license, and package metadata. The launcher has no install script and therefore
does not download or execute code during installation.

The GitHub release also contains `tailwind-doctor-vscode_VERSION.vsix`. It
contains the dependency-free VS Code client, extension metadata, README, and
license; the CLI remains a separate installation selected by
`tailwindDoctor.path`.

## npm trust

Each package trusts the `release.yml` workflow in
`Cyberlane/tailwind-doctor`, restricted to the `npm` GitHub environment and the
`npm publish` action. The workflow requests `id-token: write`; npm exchanges the
short-lived GitHub OIDC token and records package provenance automatically.
There is no long-lived npm token in the repository or Actions secrets.

Platform packages publish first, followed by `tw-doctor` and then
`tailwind-doctor`, so every dependency exists before a consumer can resolve it.
The publish script checks whether an immutable package version already exists
before acting, which makes a failed workflow safely resumable without trying to
overwrite a published version.

## Version preparation

The following must all carry the same version before tagging:

- every `npm/**/package.json` intended for publication;
- the launcher's optional dependency versions;
- the alias package's `tw-doctor` dependency;
- the VS Code `package.json` and VSIX manifest;
- release notes under `docs/release-notes/`.

The Pages source deliberately contains a release-version placeholder rather
than a committed version number. `scripts/build-site.sh` fills it from the
verified release tag and writes `release-version.txt` into the deployment.

Validate the version and complete distribution locally:

```bash
scripts/check-release-version.sh vMAJOR.MINOR.PATCH
scripts/test-release.sh
scripts/test-site.sh vMAJOR.MINOR.PATCH
```

`scripts/test-release.sh` cross-compiles all supported binaries, verifies
archive checksums, packs the npm packages, installs the host package from local
tarballs, runs both command names, and validates the VSIX archive and manifest.

## Release sequence

1. Require a clean `main` at the commit that will be released.
2. Run `gofmt -l .`, `go vet ./...`, `go test ./...`, `go test -race ./...`,
   `npm test --prefix npm`, `npm test --prefix editors/vscode`,
   `editors/neovim/test.sh`,
   `scripts/test-release.sh`, the extraction-accuracy
   gate, Mori review, and the public-boundary/history audit.
3. Push the signed commits and require green CI and CodeQL.
4. Create and push an annotated GPG-signed `vMAJOR.MINOR.PATCH` tag.
5. Let `.github/workflows/release.yml` publish npm packages and the GitHub
   release from the signed tag, then call the Pages deployment with that tag.
6. Download every release asset into a fresh directory, verify `SHA256SUMS` and
   GitHub attestations, install both npm names in clean projects, and compare
   their reported version with the tag. Confirm the Pages job passed and the
   live `release-version.txt` equals the tag.

The Pages workflow also runs for direct site changes on `main` and manually
published releases. It always resolves GitHub's latest published release and
refuses an explicit older tag, so rerunning an old release cannot silently
downgrade the site. If publication succeeds but Pages deployment fails, rerun
the Pages workflow with the latest release tag.

## Versioning

Semantic Versioning starts at `v0.1.0`. Before 1.0, rule additions and scoring
changes are minor bumps, and any change that moves a user's score is called out
prominently in release notes. Newly introduced rules remain disabled for one
minor release under the rule-stability policy.
