import type { IconNode } from 'morphicons/vue'

export const chevronExpandedPath = 'M6 9C8 11 10 13 12 15C14 13 16 11 18 9'

export const chevronCollapsedPath = 'M9 18C11 16 13 14 15 12C13 10 11 8 9 6'

const asIcon = (d: string): IconNode => [['path', { d }]]

export const chevronExpandedIcon = asIcon(chevronExpandedPath)

export const chevronCollapsedIcon = asIcon(chevronCollapsedPath)
