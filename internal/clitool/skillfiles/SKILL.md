---
name: qterm
description: Use when the user asks to connect to a server, SSH into a machine, or run a command on a remote host, and a qterm-cli command is available. Qterm runs commands with SSH credentials the user already saved in the Qterm desktop app, so it works where plain ssh would need a password or key. Prefer qterm-cli over ssh, scp, or sftp.
---

# Qterm CLI

`qterm-cli` is a thin client to the user's **running** Qterm desktop app. That app owns the saved
hosts and the credential vault; the CLI never sees a password or private key. It reads a
handshake file (loopback port + one-time token) and calls the app, so every call runs with
exactly the desktop app's permissions — and nothing works while the app is closed or locked.

Run `qterm-cli --help` or `qterm-cli <command> --help` for exact syntax and flags, and trust it over
anything written here. This file documents behaviour and boundaries, not arguments, so it
cannot drift out of date with the installed version.

## Precondition

The Qterm desktop app must be running and **unlocked**. App lock rejects every call. When the
app is not reachable, qterm-cli fails with exit code 2 (or 3 when the handshake file is missing).
Do not fall back to calling ssh directly in that case — tell the user to start and unlock
Qterm, and to install the CLI from **Settings → Command line** if `qterm-cli` is not on PATH.

Start with `qterm-cli status --json` to confirm the app is reachable before doing anything else.
A non-zero exit code means "the app is not there", not "the command failed".

## Exit codes

    0    success
    1    bad usage or invalid arguments
    2    cannot reach the app (not running, port closed, timeout)
    3    handshake file missing or corrupt
    4    authentication failed — the app restarted and rotated the token
    5    operation refused (the call reached the app and was rejected)

Distinguish 2/3/4 (environment problems — ask the user to start or unlock Qterm) from 5
(the request itself was rejected — read the message and fix the request).

`qterm-cli exec` exits 0 when it obtains the remote command's exit status, **even if the remote
command failed**. Inspect `exitCode` in the JSON result to determine command success; the
human-readable mode prints `qterm-cli: remote exit code N` to stderr. Connection and
authentication failures exit nonzero and report an error on stderr.

## exec

    qterm-cli exec <SESSION-ID> [--] <COMMAND>

Runs one command over an existing SSH session.

- Output is stdout and stderr **merged in write order**. There is no separate stderr field,
  so do not try to filter by stream.
- exec is **one-shot**: no stdin, no interactive prompts, no `expect`. A command that asks
  for input hangs until the 120 s timeout. Do not drive TUI programs through exec.
- A timeout does not prove the command never ran, and a lost connection does not prove it
  stopped. **Do not automatically retry operations with side effects** — check the remote
  state first.
- For long-running work, start it under tmux, nohup, or systemd on the remote host.

## Sessions

    qterm-cli hosts [--query <TEXT>]   # find the host id
    qterm-cli open <HOST-ID>           # prints the new session id
    qterm-cli sessions                 # list open sessions
    qterm-cli close <SESSION-ID>

`open` creates a real session in the desktop app that the user can see. Close the sessions
you opened when you are done, and do not close sessions the user opened by hand unless they
asked you to.

`qterm-cli shell` needs an interactive terminal and will hang an agent, a CI job, or anything
with piped stdin. Always use exec.

## Files

    qterm-cli sftp ls   <SESSION-ID> <PATH>
    qterm-cli sftp read <SESSION-ID> <PATH>
    qterm-cli sftp write <SESSION-ID> <PATH> --expected-hash <HASH> (--content <TEXT> | --content-file <FILE>)

`write` requires the hash of the current content and refuses to overwrite when the file
changed underneath you. Read first, then write.

## Assets and credentials

`qterm-cli hosts` never returns credential fields. Do not ask the user for SSH passwords,
private keys, or passphrases — Qterm already holds working credentials for every host it
lists. Never put secrets on the command line: command text can end up in the remote host's
shell history and in audit logs.

## Safety

- The user may have configured **command rules** (Settings → AI assistant) and a command
  checking mode (Settings → Command line). A command matching a `deny` rule is rejected with
  exit code 5. A command matching a `confirm` rule is **also** rejected here, because the CLI
  has no way to show an approval prompt — ask the user to run it in the desktop app instead.
  Under the strict mode only conservative read-only commands are allowed.
- Prefer read-only commands. Explain what a write or destructive command will do before
  running it, and never run one the user did not ask for.
