# tailwind-doctor

An alias for [`tw-doctor`](https://www.npmjs.com/package/tw-doctor), the CLI that measures design-system debt in Tailwind class lists and reports a project-wide Design System Health Score.

`npx <name>` resolves a package named `<name>`, so a `bin` alias declared inside `tw-doctor` is not enough to make `npx tailwind-doctor` work. This package exists so both invocation names do. It is published in lockstep with `tw-doctor` at the same version.

## Not released yet

Version `0.0.0` holds the package name and contains no binary. Running it prints a notice and exits with code `2`.

Until the first release, build from source with a Go 1.26 toolchain:

```bash
go run ./cmd/tw-doctor .
```

## Trademark

"Tailwind" is a trademark of Tailwind Labs. This project is not affiliated with or endorsed by Tailwind Labs.

## License

MIT
