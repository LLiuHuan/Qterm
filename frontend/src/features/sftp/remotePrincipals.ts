export interface RemotePrincipalIndex {
  users: ReadonlyMap<number, string>
  groups: ReadonlyMap<number, string>
}

const EMPTY_INDEX: RemotePrincipalIndex = { users: new Map(), groups: new Map() }

export function toPrincipalIndex(
  users: ReadonlyArray<{ id: number; name: string }>,
  groups: ReadonlyArray<{ id: number; name: string }>,
): RemotePrincipalIndex {
  const userIndex = new Map<number, string>()
  for (const user of users) {
    if (typeof user?.id !== 'number' || !user.name) continue
    if (!userIndex.has(user.id)) userIndex.set(user.id, user.name)
  }
  const groupIndex = new Map<number, string>()
  for (const group of groups) {
    if (typeof group?.id !== 'number' || !group.name) continue
    if (!groupIndex.has(group.id)) groupIndex.set(group.id, group.name)
  }
  return { users: userIndex, groups: groupIndex }
}

export function principalLabel(
  index: ReadonlyMap<number, string>,
  id: number | undefined,
  unknownLabel: string,
): string {
  const numeric = typeof id === 'number' && Number.isFinite(id) ? id : undefined
  const name = numeric === undefined ? undefined : index.get(numeric)
  if (numeric === undefined) return name ?? unknownLabel
  return name ? `${name}（${numeric}）` : String(numeric)
}

export const emptyPrincipalIndex = EMPTY_INDEX
