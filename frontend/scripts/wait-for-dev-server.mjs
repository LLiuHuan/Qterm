import process from 'node:process'
import { setTimeout as delay } from 'node:timers/promises'
import { URL } from 'node:url'

const serverUrl = process.env.FRONTEND_DEVSERVER_URL || 'http://localhost:9245'
const probeUrl = new URL(serverUrl)

if (probeUrl.hostname === 'localhost') probeUrl.hostname = '127.0.0.1'
const timeoutMs = 30_000
const retryDelayMs = 200
const deadline = Date.now() + timeoutMs
let lastError = ''

while (Date.now() < deadline) {
  try {
    const response = await fetch(probeUrl, { signal: AbortSignal.timeout(1_000) })
    if (response.ok) process.exit(0)
    lastError = `HTTP ${response.status}`
  } catch (error) {
    lastError = error instanceof Error ? error.message : String(error)
  }
  await delay(retryDelayMs)
}

process.stderr.write(
  `Frontend dev server did not become ready at ${serverUrl} within ${timeoutMs / 1_000}s: ${lastError}\n`,
)
process.exit(1)
