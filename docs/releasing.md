# Releasing

## npm packages

Two packages ship, always at the same version:

- `tw-doctor` — the real launcher. From the first release it declares the
  platform binary packages as `optionalDependencies` and exposes both the
  `tw-doctor` and `tailwind-doctor` commands for a local install.
- `tailwind-doctor` — an alias so `npx tailwind-doctor` works. `npx <name>`
  resolves a *package* named `<name>`, so a `bin` alias inside `tw-doctor` is
  not enough on its own.

Publish `tw-doctor` first once `tailwind-doctor` depends on it, so the
dependency resolves. Verify the tarball contents before publishing:

```bash
cd npm/tw-doctor       && npm pack --dry-run
cd npm/tailwind-doctor && npm pack --dry-run
```

Then, from each directory:

```bash
npm publish --access public
```

### The 0.0.0 placeholders

Version `0.0.0` reserves both names before the tool is ready to distribute. The
placeholders contain no binary and no install-time download; running either one
prints a notice explaining that no release exists yet and exits with code `2`,
the CLI's operational-error code.

`tailwind-doctor@0.0.0` deliberately does *not* depend on `tw-doctor@0.0.0`, so
the two can be published in either order and neither placeholder drags a second
non-functional package into an install. The dependency is added when the real
launcher ships.

## Versioning

Semantic Versioning from the first tag. Before 1.0, rule additions and scoring
changes are minor bumps, and any change that moves a user's score is called out
prominently in the release notes.
