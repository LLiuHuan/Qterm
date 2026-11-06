import type { Group, Snippet, SnippetInput } from '../../bindings/changeme/internal/model/models'
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
    },
    async renameGroup(id, name) {
      const { RenameSnippetGroup } = await import('../../bindings/changeme/internal/app/service')
      return RenameSnippetGroup(id, name)
    },
    async removeGroup(id) {
      const { DeleteSnippetGroup } = await import('../../bindings/changeme/internal/app/service')
      await DeleteSnippetGroup(id)
    },
    async listGroups() {
      const { ListSnippetGroups } = await import('../../bindings/changeme/internal/app/service')
      return (await ListSnippetGroups()) ?? []
    },
    async createGroup(name) {
      const { CreateSnippetGroup } = await import('../../bindings/changeme/internal/app/service')
      return CreateSnippetGroup(name)
    },
    async list() {
      const { ListSnippets } = await import('../../bindings/changeme/internal/app/service')
      return (await ListSnippets()) ?? []
    },
    async create(input) {
      const { CreateSnippet } = await import('../../bindings/changeme/internal/app/service')
      return CreateSnippet(input)
    },
    async update(id, input) {
      const { UpdateSnippet } = await import('../../bindings/changeme/internal/app/service')
      return UpdateSnippet(id, input)
    },
    async remove(id) {
      const { DeleteSnippet } = await import('../../bindings/changeme/internal/app/service')
      await DeleteSnippet(id)
    },
  }
}
