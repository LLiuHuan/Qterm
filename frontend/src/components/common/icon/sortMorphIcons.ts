import type { IconNode } from 'morphicons/vue'

export const sortNameAscPath =
  'M3 16C4.3333 17.3333 5.6667 18.6667 7 20C8.3333 18.6667 9.6667 17.3333 11 16M7 20C7 14.6667 7 9.3333 7 4M20 8C18.3333 8 16.6667 8 15 8M15 10C15 8.8333 15 7.6667 15 6.5C15 5.1193 16.1193 4 17.5 4C18.8807 4 20 5.1193 20 6.5C20 7.6667 20 8.8333 20 10M15 14C16.6667 14 18.3333 14 20 14C18.3333 16 16.6667 18 15 20C16.6667 20 18.3333 20 20 20'

export const sortNameDescPath =
  'M3 8C4.3333 6.6667 5.6667 5.3333 7 4C8.3333 5.3333 9.6667 6.6667 11 8M7 4C7 9.3333 7 14.6667 7 20M15 4C16.6667 4 18.3333 4 20 4C18.3333 6 16.6667 8 15 10C16.6667 10 18.3333 10 20 10M15 20C15 18.8333 15 17.6667 15 16.5C15 15.1193 16.1193 14 17.5 14C18.8807 14 20 15.1193 20 16.5C20 17.6667 20 18.8333 20 20M20 18C18.3333 18 16.6667 18 15 18'

export const sortCustomPath =
  'M3 16C4.3333 17.3333 5.6667 18.6667 7 20C8.3333 18.6667 9.6667 17.3333 11 16M7 20C7 14.6667 7 9.3333 7 4M21 8C19.6667 6.6667 18.3333 5.3333 17 4C15.6667 5.3333 14.3333 6.6667 13 8M17 4C17 9.3333 17 14.6667 17 20'

const asIcon = (d: string): IconNode => [['path', { d }]]

export const sortNameAscIcon = asIcon(sortNameAscPath)

export const sortNameDescIcon = asIcon(sortNameDescPath)

export const sortCustomIcon = asIcon(sortCustomPath)

export type SortMorphMode = 'name-asc' | 'name-desc' | 'custom'

export function sortMorphIcon(mode: SortMorphMode): IconNode {
  if (mode === 'name-asc') return sortNameAscIcon
  if (mode === 'name-desc') return sortNameDescIcon
  return sortCustomIcon
}
