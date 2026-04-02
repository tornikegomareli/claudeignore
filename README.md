# claudeignore

CC hook that blocks file access based on `.claudeignore` patterns.

Claude reads files to understand your codebase. but it doesn't need to read sensitive files or deprecated features, or features which are large you don't need them and it takes lot of tokens and fills context without reason. When it does, it wastes tokens

cc has built-in deny rules in `.claude/settings.json`:

```json
{
  "permissions": {
    "deny": ["Read(./node_modules/**)"]
  }
}
```

This works, but has some problems for me

- **Per-tool syntax** — you write `Read(./node_modules/**)`, `Edit(./node_modules/**)`, `Write(./node_modules/**)` separately for each tool
- **No shared file** — rules live in `settings.json`, mixed with other config. You can't commit a simple ignore file that teammates can read and edit


claudeignore is simple a [PreToolUse hook](https://docs.anthropic.com/en/docs/claude-code/hooks), before cc reads, edits, writes, globs, or greps a file, the hook checks the path against your `.claudeignore`. If it matches, the operation is blocked.

The `.claudeignore` file uses **gitignore syntax**, the same patterns you already know.

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

Create a `.claudeignore` in your project and put there anything you don't want claude to touch.


## Commands

```bash
claudeignore setup          # one-time: register hook globally
claudeignore update         # update to latest version
claudeignore version        # show version
```

## License

[MIT](LICENSE)
