import { onScopeDispose, ref } from 'vue'
import { toast } from 'vue-sonner'
import { hostErrorMessage, type GroupRecord, type HostGateway } from '@/services/hosts.ts'
import { useI18n } from '@/locales'

interface GroupEditorOptions {
  hostGateway: Pick<HostGateway, 'createGroup' | 'updateGroup'>
  onSaved: () => Promise<void>
}

export function useGroupEditor({ hostGateway, onSaved }: GroupEditorOptions) {
  const { t } = useI18n()
  const groupDialogOpen = ref(false)
  const editingGroupId = ref<string | null>(null)
  const groupName = ref('')
  const groupParentId = ref<string | undefined>(undefined)
  const groupFormError = ref('')
  const savingGroup = ref(false)
  let disposed = false
  let editorVersion = 0

  function openCreateGroupDialog(parentId?: string): void {
    if (disposed || savingGroup.value) return
    editorVersion += 1
    editingGroupId.value = null
    groupName.value = ''
    groupParentId.value = parentId
    groupFormError.value = ''
    groupDialogOpen.value = true
  }

  function openEditGroupDialog(group: { id: string; name: string }): void {
    if (disposed || savingGroup.value) return
    editorVersion += 1
    editingGroupId.value = group.id
    groupName.value = group.name
    groupParentId.value = undefined
    groupFormError.value = ''
    groupDialogOpen.value = true
  }

  function closeGroupDialog(): void {
    editorVersion += 1
    groupDialogOpen.value = false
    editingGroupId.value = null
    groupFormError.value = ''
  }

  async function saveGroup(): Promise<GroupRecord | null> {
    if (disposed || savingGroup.value || !groupDialogOpen.value) return null
    const name = groupName.value.trim()
    if (!name) {
      groupFormError.value = t('shell.error.groupNameRequired')
      return null
    }
    const id = editingGroupId.value
    const parentId = groupParentId.value
    const version = editorVersion
    savingGroup.value = true
    groupFormError.value = ''
    try {
      const saved = id ? await hostGateway.updateGroup(id, name) : await hostGateway.createGroup(name, parentId)
      if (disposed) return null
      if (version === editorVersion) closeGroupDialog()
      toast.success(id ? t('shell.success.groupUpdated') : t('shell.success.groupCreated'))
      await onSaved()
      return saved
    } catch (error) {
      if (!disposed) toast.error(hostErrorMessage(error, t('shell.error.saveGroup')))
      return null
    } finally {
      savingGroup.value = false
    }
  }

  onScopeDispose(() => {
    disposed = true
    editorVersion += 1
  })
  return {
    groupDialogOpen,
    editingGroupId,
    groupName,
    groupFormError,
    savingGroup,
    openCreateGroupDialog,
    openEditGroupDialog,
    closeGroupDialog,
    saveGroup,
  }
}
