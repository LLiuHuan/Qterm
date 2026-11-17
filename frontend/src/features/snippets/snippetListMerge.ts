import type { Snippet } from '../../services/snippets'

export interface SnippetGroupOption {
  id: string
  name: string
}

function sameSnippetId(item: Snippet, id: string): boolean {
  return item.id === id
}

export function upsertSnippet(list: readonly Snippet[], record: Snippet): Snippet[] {
  const index = list.findIndex((item) => sameSnippetId(item, record.id))
  if (index >= 0) {
    const next = list.slice()
    next[index] = record
    return next
  }
  return [...list, record]
}

export function removeSnippet(list: readonly Snippet[], id: string): Snippet[] {
  const index = list.findIndex((item) => sameSnippetId(item, id))
  if (index < 0) return list as Snippet[]
  return [...list.slice(0, index), ...list.slice(index + 1)]
}

export function upsertSnippetGroup(
  list: readonly SnippetGroupOption[],
  record: SnippetGroupOption,
): SnippetGroupOption[] {
  const index = list.findIndex((item) => item.id === record.id)
  if (index >= 0) {
    const next = list.slice()
    next[index] = record
    return next
  }
  return [...list, record]
}

export function removeSnippetGroup(
  groups: readonly SnippetGroupOption[],
  snippets: readonly Snippet[],
  id: string,
): { groups: SnippetGroupOption[]; snippets: Snippet[] } {
  const remaining = groups.filter((item) => item.id !== id)
  const orphaned = snippets.some((item) => item.groupId === id)
  return {
    groups: remaining.length === groups.length ? (groups as SnippetGroupOption[]) : remaining,

    snippets: orphaned
      ? snippets.map((item) => (item.groupId === id ? { ...item, groupId: '' } : item))
      : (snippets as Snippet[]),
  }
}

export function renameSnippetGroupInSnippets(snippets: readonly Snippet[], groupId: string, name: string): Snippet[] {
  let changed = false
  const next = snippets.map((item) => {
    if (item.groupId !== groupId || item.group === name) return item
    changed = true
    return { ...item, group: name }
  })
  return changed ? next : (snippets as Snippet[])
}

export function groupKeyOfSnippet(item: Snippet): string {
  return item.pinned ? '__pinned__' : item.groupId || '__ungrouped__'
}
