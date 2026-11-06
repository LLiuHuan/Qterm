import { createSftpGateway } from './sftp'
import { requestTransferConflict, requestRemoteDirectory, transferCopyName, SkipTransfer } from './transfer-conflicts'
import type { ZmodemDownloadTarget, ZmodemUploadSource } from './zmodem'

let dialogQueue: Promise<unknown> = Promise.resolve()
function withDialog<T>(signal: AbortSignal, run: () => Promise<T>): Promise<T | null> {
  const result = dialogQueue.then(() => (signal.aborted ? null : run()))
  dialogQueue = result.catch(() => {})
  return result
}

export async function selectZmodemUpload(
  signal: AbortSignal,
  remote?: { sessionId: string; directory: string },
): Promise<ZmodemUploadSource | null> {
  const api = await import('../../bindings/changeme/internal/app/service')
  return withDialog(signal, async () => {
    const file = await api.SelectZmodemUpload()
    if (!file.token) return null
    if (signal.aborted) {
      await api.ReleaseZmodemFile(file.token)
      return null
    }
    let name = file.name
    let overwrite = false
    try {
      if (remote) {
        const directory = remote.directory || (await requestRemoteDirectory(signal, 'rz'))
        if (!directory || signal.aborted) {
          await api.ReleaseZmodemFile(file.token)
          return null
        }
        const entries = await createSftpGateway().listRemoteDirectory(remote.sessionId, directory)
        if (signal.aborted) {
          await api.ReleaseZmodemFile(file.token)
          return null
        }
        const names = new Set(entries.map((entry) => entry.name))
        if (names.has(name)) {
          const action = await requestTransferConflict(name, signal)
          if (action === 'cancel' || action === 'stop' || signal.aborted) {
            await api.ReleaseZmodemFile(file.token)
            return null
          }
          overwrite = action === 'overwrite'
          if (action === 'copy') {
            let number = 1
            while (names.has(transferCopyName(file.name, number))) number++
            name = transferCopyName(file.name, number)
          }
        }
      }
    } catch (error) {
      await api.ReleaseZmodemFile(file.token)
      throw error
    }
    return {
      name,
      overwrite,
      size: file.size,
      async read(offset, length) {
        const encoded = await api.ReadZmodemFile(file.token, offset, length)
        return Uint8Array.from(atob(encoded ?? ''), (char) => char.charCodeAt(0))
      },
      close: () => api.ReleaseZmodemFile(file.token),
    }
  })
}

export async function selectZmodemDestination(
  info: { name: string; size: number },
  signal: AbortSignal,
): Promise<ZmodemDownloadTarget | null> {
  const api = await import('../../bindings/changeme/internal/app/service')
  return withDialog(signal, async () => {
    let file = await api.SelectZmodemDownload(info.name, info.size)
    if (!file.token) return null
    if (signal.aborted) {
      await api.ReleaseZmodemFile(file.token)
      return null
    }
    try {
      if (file.conflict) {
        const action = await requestTransferConflict(file.name, signal)
        if (signal.aborted || action === 'stop') {
          await api.ReleaseZmodemFile(file.token)
          return null
        }
        if (action === 'cancel') throw new SkipTransfer()
        file = await api.ResolveZmodemDownload(file.token, action)
      }
    } catch (error) {
      await api.ReleaseZmodemFile(file.token)
      throw error
    }
    let offset = 0
    return {
      async write(data) {
        for (let start = 0; start < data.length; start += 32768) {
          if (signal.aborted) throw new Error('传输已取消')
          const chunk = data.subarray(start, start + 32768)
          await api.WriteZmodemFile(file.token, offset, btoa(String.fromCharCode(...chunk)))
          offset += chunk.length
        }
      },
      async finish() {
        if (signal.aborted) throw new Error('传输已取消')
        await api.FinishZmodemFile(file.token)
      },
      close: () => api.ReleaseZmodemFile(file.token),
    }
  })
}
