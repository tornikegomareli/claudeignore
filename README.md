# claudeignore

A [Claude Code](https://docs.anthropic.com/en/docs/claude-code) hook that blocks file access based on `.claudeignore` patterns.

## The problem

Claude Code reads files to understand your codebase. But it doesn't need to read `node_modules/`, `dist/`, or `.env`. When it does, it wastes tokens on irrelevant content and exposes sensitive files.

## Why not use `permissions.deny`?

Claude Code has built-in deny rules in `.claude/settings.json`:

```json
{
  "permissions": {
    "deny": ["Read(./node_modules/**)"]
  }
}
```

This works, but has real limitations:

- **Per-tool syntax** — you write `Read(./node_modules/**)`, `Edit(./node_modules/**)`, `Write(./node_modules/**)` separately for each tool
- **No shared file** — rules live in `settings.json`, mixed with other config. You can't commit a simple ignore file that teammates can read and edit
- **Not portable** — you configure it per-project in `.claude/settings.json`. With `claudeignore`, one global setup covers every project that has a `.claudeignore` file

## How claudeignore works

It's a [PreToolUse hook](https://docs.anthropic.com/en/docs/claude-code/hooks). Before Claude Code reads, edits, writes, globs, or greps a file, the hook checks the path against your `.claudeignore`. If it matches, the operation is blocked.

The `.claudeignore` file uses **gitignore syntax** — the same patterns you already know.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/tornikegomareli/claudeignore/main/install.sh | sh
```

Or build from source:

```bash
go install github.com/tornikegomareli/claudeignore@latest
claudeignore setup
```

## Use

Create a `.claudeignore` in your project:

```
node_modules/
dist/
build/
.env
.env.*
*.min.js
```

That's it. No restart, no init command. Changes take effect on the next tool call.

## How it works under the hood

1. You run `claudeignore setup` once (the install script does this automatically)
2. This registers a PreToolUse hook in `~/.claude/settings.json`
3. Every time Claude Code calls Read, Edit, Write, Glob, or Grep, the hook runs
4. The hook walks up from the file path to find the nearest `.claudeignore`
5. If the file matches a pattern, the hook exits with code 2 (block)
6. Claude sees the block and moves on — no tokens wasted

Hook latency is ~3ms.

## Commands

```bash
claudeignore setup          # one-time: register hook globally
claudeignore test <path>    # check if a path would be blocked
claudeignore status         # show hook registration and pattern count
claudeignore update         # update to latest version
claudeignore version        # show version
```

## .claudeignore syntax

Same as `.gitignore`:

```
# comments
node_modules/          # directory and everything in it
*.min.js               # glob pattern
.env                   # specific file
.env.*                 # wildcard
/config/secrets.json   # rooted to project (only matches at this path)
!important.env         # negation — allow this even if matched above
```

## Uninstall

```bash
rm ~/.local/bin/claudeignore
```

Then remove the `claudeignore hook` entry from `~/.claude/settings.json`.

## License

MIT
