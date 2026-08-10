# tw-doctor

Tailwind Doctor is a fast, read-only-by-default CLI that measures design-system
debt in Tailwind class lists and reports a project-wide Design System Health
Score with file-level evidence.

## Run

```bash
npx tw-doctor .
npx tw-doctor --json .
npx tw-doctor --sarif .
npx tw-doctor --fix .
```

The package installs one prebuilt Go binary for the current platform through an
optional dependency. Supported targets are macOS arm64/x64, Linux arm64/x64,
and Windows x64. No install script downloads or executes code.

The `tailwind-doctor` npm package is a lockstep alias for this package.

To build from source with a Go 1.26 toolchain:

```bash
go install github.com/Cyberlane/tailwind-doctor/cmd/tw-doctor@v0.2.0
```

## Trademark

"Tailwind" is a trademark of Tailwind Labs. This project is not affiliated with or endorsed by Tailwind Labs.

## License

MIT
