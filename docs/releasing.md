# Releasing

## npm packages

Two packages ship, always at the same version:

- `tw-doctor` — the real launcher. From the first release it declares the
  platform binary packages as `optionalDependencies` and exposes both the
  `tw-doctor` and `tailwind-doctor` commands for a local install.
- `tailwind-doctor` — an alias so `npx tailwind-doctor` works. `npx <name>`
  resolves a *package* named `<name>`, so a `bin` alias inside `tw-doctor` is
  not enough on its own.

## Platform binary packages

Prebuilt binaries ship as one package per platform, declared by the launcher as
`optionalDependencies` and selected by npm through their `os` and `cpu` fields,
so an install downloads one binary rather than all of them.

These packages are **scoped**: `@tw-doctor/darwin-arm64`, `@tw-doctor/linux-x64`,
and so on. The scope is what makes the scheme safe. Under an unscoped naming
convention such as `tw-doctor-linux-arm64`, anyone can register a platform name
before this project does; adding that platform later would then resolve an
`optionalDependency` to a package under someone else's control, whose install
scripts run on every user's machine. A scope is a namespace this project owns
outright, so no such package can exist without being published here.

The two user-facing names, `tw-doctor` and `tailwind-doctor`, stay unscoped —
`npx <name>` has to find them by bare name.

Scoped packages default to restricted access, so every publish needs
`--access public` explicitly. A private-by-accident platform package breaks
installs for everyone except the publisher.

## Publish order

Publish `tw-doctor` first once `tailwind-doctor` depends on it, so the
dependency resolves. Platform packages must exist before the launcher that lists
them, so the full order at release time is: platform packages, then `tw-doctor`,
then `tailwind-doctor`.

Verify the tarball contents before publishing:

```bash
cd npm/tw-doctor       && npm pack --dry-run
cd npm/tailwind-doctor && npm pack --dry-run
```

Then, from each directory:

```bash
npm publish --access public
```

### The 0.0.0 placeholders

Both names are published at `0.0.0`, reserving them before the tool is ready to
distribute. The placeholders contain no binary and no install-time download;
running either one prints a notice explaining that no release exists yet and
exits with code `2`, the CLI's operational-error code.

`tailwind-doctor@0.0.0` deliberately does *not* depend on `tw-doctor@0.0.0`, so
the two can be published in either order and neither placeholder drags a second
non-functional package into an install. The dependency is added when the real
launcher ships.

## Versioning

Semantic Versioning from the first tag. Before 1.0, rule additions and scoring
changes are minor bumps, and any change that moves a user's score is called out
prominently in the release notes.
