const PROCESS_TERMINATION_COMMANDS = new Set(['kill', 'pkill', 'killall', 'taskkill', 'tskill', 'stop-process'])
const COMMAND_WRAPPERS = new Set(['sudo', 'doas', 'env', 'command', 'builtin', 'exec', 'nohup'])
const WRAPPER_VALUE_OPTIONS: Record<string, Set<string>> = {
  sudo: new Set(['-u', '--user', '-g', '--group', '-h', '--host', '-p', '--prompt']),
  doas: new Set(['-u', '-C']),
  env: new Set(['-u', '--unset', '-C', '--chdir', '--argv0']),
  exec: new Set(['-a']),
}

function commandSegments(input: string): string[][] {
  const segments: string[][] = []
  let words: string[] = []
  let word = ''
  let quote = ''
  let escaped = false
  const finishWord = () => {
    if (word) words.push(word)
    word = ''
  }
  for (const char of input) {
    if (escaped) {
      word += char
      escaped = false
    } else if (char === '\\' && quote !== "'") {
      escaped = true
    } else if (quote) {
      if (char === quote) quote = ''
      else word += char
    } else if (char === '"' || char === "'") {
      quote = char
    } else if (';&|\r\n'.includes(char)) {
      finishWord()
      if (words.length) segments.push(words)
      words = []
    } else if (/\s/.test(char)) {
      finishWord()
    } else {
      word += char
    }
  }
  finishWord()
  if (words.length) segments.push(words)
  return segments
}

function isProcessTermination(words: string[]): boolean {
  let index = 0
  while (index < words.length) {
    const word = words[index]!

    if (/^[A-Za-z_][A-Za-z0-9_]*=/.test(word)) {
      index += 1
      continue
    }
    const executable = word
      .split('/')
      .at(-1)!
      .toLowerCase()
      .replace(/\.exe$/, '')
    if (PROCESS_TERMINATION_COMMANDS.has(executable)) return true
    if (!COMMAND_WRAPPERS.has(executable)) return false
    index += 1
    while (index < words.length && words[index]!.startsWith('-')) {
      const option = words[index++]!
      if (option === '--') break
      if (WRAPPER_VALUE_OPTIONS[executable]?.has(option)) index += 1
    }
  }
  return false
}

export function isCommandCompletionCandidate(command: string): boolean {
  return !commandSegments(command).some(isProcessTermination)
}
