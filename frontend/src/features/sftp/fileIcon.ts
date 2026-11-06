import type { SftpIconName } from './fileIconAssets'
import { materialFileExtensionsToIcons, materialFileNamesToIcons } from './materialFileExtensionsToIcons'
import { materialFolderNamesToIcons } from './materialFolderNamesToIcons'

export type SftpIconEntryType = 'file' | 'directory' | 'symlink' | 'other'

const fileIcons = Object.fromEntries(
  Object.entries(materialFileExtensionsToIcons).map(([name, icon]) => [name.toLowerCase(), icon]),
) as Record<string, string>

const fileNameIcons = Object.fromEntries(
  Object.entries(materialFileNamesToIcons).map(([name, icon]) => [name.toLowerCase(), icon]),
) as Record<string, string>

const folderIcons = Object.fromEntries(
  Object.entries(materialFolderNamesToIcons).map(([name, icon]) => [name.toLowerCase(), icon]),
) as Record<string, string>

const ICON_ALIASES: Record<string, string> = {
  md: 'markdown',
  mdc: 'markdown',
  file: 'document',
  advpl_include: 'advpl',
  advpl_prw: 'advpl',
  advpl_ptm: 'advpl',
  advpl_tlpp: 'advpl',
  'angular-component': 'angular',
  'angular-directive': 'angular',
  'nest-controller': 'nest',
  'nest-decorator': 'nest',
  'nest-filter': 'nest',
  'nest-gateway': 'nest',
  'nest-guard': 'nest',
  'nest-middleware': 'nest',
  'nest-module': 'nest',
  'nest-pipe': 'nest',
  'nest-resolver': 'nest',
  'nest-service': 'nest',
  nrwl: 'nx',
  shaderlab: 'shader',
  'folder-ngrx-effects': 'folder-ngrx-store',
  'folder-ngrx-entities': 'folder-ngrx-store',
  'folder-ngrx-selectors': 'folder-ngrx-store',
  'folder-ngrx-state': 'folder-ngrx-store',
  'folder-redux-actions': 'folder-redux-reducer',
}

function availableIcon(name: string, fallback: SftpIconName): SftpIconName {
  return name || fallback
}

function basename(name: string): string {
  return name.trim().split(/[\\/]/).pop()?.toLowerCase() ?? ''
}

export class SftpFileIconResolver {
  static resolve(name: string, type: SftpIconEntryType = 'file'): SftpIconName {
    if (type === 'symlink') return availableIcon('folder-link', 'folder')
    if (type === 'other') return availableIcon('file', 'document')

    const baseName = basename(name)
    if (type === 'directory') {
      const mapped = folderIcons[baseName] ?? folderIcons.folder
      return availableIcon(ICON_ALIASES[mapped] ?? mapped, 'folder')
    }
    if (!baseName) return 'document'

    const exact = fileNameIcons[baseName] ?? fileIcons[baseName]
    if (exact) return availableIcon(ICON_ALIASES[exact] ?? exact, 'document')

    const dotIndex = baseName.indexOf('.')
    if (dotIndex === 0 && baseName.startsWith('.env')) {
      return availableIcon(fileIcons['.env'] ?? 'tune', 'document')
    }

    const compound = baseName.includes('.') ? baseName.slice(baseName.indexOf('.') + 1) : ''
    const extension = baseName.includes('.') ? (baseName.split('.').pop() ?? '') : ''
    const mapped = fileIcons[compound] ?? fileIcons[extension]
    if (mapped) return availableIcon(ICON_ALIASES[mapped] ?? mapped, 'document')
    return 'document'
  }
}

export function fileIconName(name: string): SftpIconName {
  return SftpFileIconResolver.resolve(name)
}
