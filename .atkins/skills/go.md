`atkins go` runs the whole lifecycle: generate, fmt, lint, test, build.

- `atkins go:fmt` - format the import blocks with `splint fix` and tidy the module. One pass, so there is no order to get wrong.
- `atkins go:test` - run the tests and write the coverage profile `pkg.cov`.
