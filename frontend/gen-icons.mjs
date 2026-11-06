import { readdirSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'

const dir = resolve('src/components/common/icon')
const files = readdirSync(dir).filter((f) => f.endsWith('.vue'))

const content =
  files
    .map((f) => {
      const name = f.replace(/\.vue$/, '')
      return `export { default as ${name} } from './${f}'`
    })
    .join('\n') + '\n'

writeFileSync(resolve(dir, 'index.ts'), content)
