# tailwind-doctor

An alias for [`tw-doctor`](https://www.npmjs.com/package/tw-doctor), the CLI that measures design-system debt in Tailwind class lists and reports a project-wide Design System Health Score.

`npx <name>` resolves a package named `<name>`, so a `bin` alias declared inside `tw-doctor` is not enough to make `npx tailwind-doctor` work. This package exists so both invocation names do. It is published in lockstep with `tw-doctor` at the same version.

## Run

```bash
npx tailwind-doctor .
```

The package delegates to `tw-doctor`, which installs one prebuilt Go binary for
the current platform without an install-time download script.

## Trademark

"Tailwind" is a trademark of Tailwind Labs. This project is not affiliated with or endorsed by Tailwind Labs.

## License

MIT
