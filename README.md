# claudeignore

CC hook that blocks file access based on .claudeignore patterns. Works at the hook level for tool calls and at the, OS kernel level to stop sub-agents and shell commands via macOS [seatbelt](https://theapplewiki.com/wiki/Dev:Seatbelt), 

**⚠️ but currently OS level protection works only on macOS.**

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

## sub-agent protection (macOS only right now)

sub-agents spawned via the Agent tool can bypass hooks. On macOS, `claudeignore wrap` launches Claude inside a kernel-level sandbox Seatbelt

<img width="3390" height="1164" alt="CleanShot 2026-04-03 at 03 45 17@2x" src="https://github.com/user-attachments/assets/9230f762-9d30-4b7b-9af3-e19a7f289ca2" />

```bash
claudeignore wrap
```

Every process in the tree, sub-agents, shell commands, everything is blocked from accessing ignored files at the OS level. There is no way around it.


> `wrap` is macOS only. Linux and Windows need their own implementations, contributions welcome. As I researched on Linux it can be used landlock LSM, and on windows detours/iat hooking, but do your own research.

## simple commands

```bash
claudeignore wrap           # launch claude inside macOS kernel sandbox
claudeignore status         # show current configuration
claudeignore update         # update to latest version
```

## License

[MIT](LICENSE)
