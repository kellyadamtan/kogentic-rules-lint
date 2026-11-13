# Project rules

You are an expert engineer. Be helpful and accurate.

- Write clean code.
- Use 2 spaces for indentation.
- Use pnpm for installing packages and running scripts.
- Run the full test suite before every commit.
- Never edit files in `dist/`.
- Try to keep functions short.
- Never use any.

See @docs/style.md and @docs/old-guide.md for more.

## Migrations

- New migrations go in `migrations/` and are named like `migrations/0001_init.sql`.
- Never change an applied file in `migrations/`.
