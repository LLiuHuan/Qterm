import type { IconNode } from 'morphicons/vue'

export const listCollapsePath = 'M3 5h8M3 12h8M3 19h8M15 5l3 3 3-3M15 19l3-3 3 3'

export const listExtendPath = 'M3 5h8M3 12h8M3 19h8M15 8l3-3 3 3M15 16l3 3 3-3'

const asIcon = (d: string): IconNode => [['path', { d }]]

export const listCollapseIcon = asIcon(listCollapsePath)

export const listExtendIcon = asIcon(listExtendPath)
