import type { Plugin } from 'vite'

export function wailsTypeScriptBindings(): Plugin {
  return {
    name: 'qterm-wails-typescript-bindings',
    enforce: 'pre',
    resolveId(source, importer) {
      if (!source.startsWith('.') && !source.startsWith('/')) return null
      if (source.includes('eventcreate')) return null
      const base = importer?.slice(0, importer.lastIndexOf('/')) ?? ''
      const candidate = source.startsWith('/') ? source : `${base}/${source}`
      if (!candidate.includes('/bindings/')) return null
      if (candidate.endsWith('.ts')) return null
      if (candidate.endsWith('/eventcreate.js')) return null
      const typed = candidate.endsWith('.js') ? candidate.slice(0, -3) + '.ts' : candidate + '.ts'
      return typed
    },
  }
}
