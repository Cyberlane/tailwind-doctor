# tw-doctor

Tailwind Doctor is a fast, read-only CLI that measures design-system debt in Tailwind class lists and reports a project-wide Design System Health Score with file-level evidence.

## Not released yet

Version `0.0.0` holds the package name and contains no binary. Running it prints a notice and exits with code `2`.

The first real release will ship prebuilt Go binaries as platform-specific optional dependencies, so that `npx tw-doctor` downloads one binary rather than all of them. The `tailwind-doctor` package is a lockstep alias published at the same version.

Until then, build from source with a Go 1.26 toolchain:

```bash
go run ./cmd/tw-doctor .
```

## Trademark

"Tailwind" is a trademark of Tailwind Labs. This project is not affiliated with or endorsed by Tailwind Labs.

## License

MIT
