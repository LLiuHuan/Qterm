import type { Group, Snippet, SnippetInput } from '../../bindings/changeme/internal/model/models'
import { emitAssetChange } from './assetEvents'
export type { Snippet, SnippetInput }

export interface SnippetGateway {
  reorder(id: string, groupId: string, targetId: string, position: 'before' | 'after'): Promise<void>
  listGroups(): Promise<Group[]>
  createGroup(name: string): Promise<Group>
  renameGroup(id: string, name: string): Promise<Group>
  removeGroup(id: string): Promise<void>
  list(): Promise<Snippet[]>
  create(input: SnippetInput): Promise<Snippet>
  update(id: string, input: SnippetInput): Promise<Snippet>
  remove(id: string): Promise<void>
}
export function createSnippetGateway(): SnippetGateway {
  return {
    async reorder(id, groupId, targetId, position) {
      const { ReorderSnippet } = await import('../../bindings/changeme/internal/app/service')
      await ReorderSnippet(id, groupId, targetId, position)

      await emitAssetChange({ kind: 'snippet', action: 'invalidate' })
    },
    async renameGroup(id, name) {
      const { RenameSnippetGroup } = await import('../../bindings/changeme/internal/app/service')
      const group = await RenameSnippetGroup(id, name)

      await emitAssetChange({ kind: 'snippetGroup', action: 'invalidate', id })
      return group
    },
    async removeGroup(id) {
      const { DeleteSnippetGroup } = await import('../../bindings/changeme/internal/app/service')
      await DeleteSnippetGroup(id)

      await emitAssetChange({ kind: 'snippetGroup', action: 'delete', id })
    },
    async listGroups() {
      const { ListSnippetGroups } = await import('../../bindings/changeme/internal/app/service')
      return (await ListSnippetGroups()) ?? []
    },
    async createGroup(name) {
      const { CreateSnippetGroup } = await import('../../bindings/changeme/internal/app/service')
      const group = await CreateSnippetGroup(name)
      await emitAssetChange({ kind: 'snippetGroup', action: 'create', record: group })
      return group
    },
    async list() {
      const { ListSnippets } = await import('../../bindings/changeme/internal/app/service')
      return (await ListSnippets()) ?? []
    },
    async create(input) {
      const { CreateSnippet } = await import('../../bindings/changeme/internal/app/service')
      const snippet = await CreateSnippet(input)
      await emitAssetChange({ kind: 'snippet', action: 'create', record: snippet })
      return snippet
    },
    async update(id, input) {
      const { UpdateSnippet } = await import('../../bindings/changeme/internal/app/service')
      const snippet = await UpdateSnippet(id, input)
      await emitAssetChange({ kind: 'snippet', action: 'update', record: snippet })
      return snippet
    },
    async remove(id) {
      const { DeleteSnippet } = await import('../../bindings/changeme/internal/app/service')
      await DeleteSnippet(id)
      await emitAssetChange({ kind: 'snippet', action: 'delete', id })
    },
  }
}
