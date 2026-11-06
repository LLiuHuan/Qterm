import { URL } from 'node:url'
import process from 'node:process'
import console from 'node:console'
import { execFileSync } from 'node:child_process'
import { mkdtempSync, readFileSync, writeFileSync, unlinkSync, rmdirSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import assert from 'node:assert/strict'

const source = readFileSync(new URL('../../internal/tunnel/directory_follow.go', import.meta.url), 'utf8')
const block = /const directoryFollowBlock = `([\s\S]*?)`/.exec(source)[1]
assert.equal(block, String.raw`PROMPT_COMMAND='printf "\033]1337;CurrentDir=%s\007" "$PWD"'` + '\n')
const home = mkdtempSync(join(tmpdir(), 'qterm-follow-'))
const rc = join(home, '.bashrc')
try {
  writeFileSync(rc, block)
  const output = execFileSync(
    '/bin/bash',
    ['--noprofile', '--norc', '-c', '. "$HOME/.bashrc"; cd /; eval "$PROMPT_COMMAND"'],
    {
      env: { ...process.env, HOME: home },
      encoding: 'utf8',
    },
  )
  assert.equal(output, '\x1b]1337;CurrentDir=/\x07')
} finally {
  unlinkSync(rc)
  rmdirSync(home)
}
console.log('Bash verification passed: exact requested configuration and dynamic PWD')
