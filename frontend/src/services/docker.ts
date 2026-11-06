import type {
  DockerImage,
  DockerImageDetail,
  DockerNetwork,
  DockerNetworkDetail,
  DockerNetworkMember,
  DockerSubnet,
  DockerVolume,
  DockerVolumeDetail,
} from '../../bindings/changeme/internal/model/models'
import type { AppError } from './errors'
import { createRuntimeGateway } from './runtime'

export type {
  DockerImage,
  DockerImageDetail,
  DockerNetwork,
  DockerNetworkDetail,
  DockerNetworkMember,
  DockerVolume,
  DockerVolumeDetail,
}

export type DockerImageDetailView = Omit<
  DockerImageDetail,
  'repoTags' | 'repoDigests' | 'exposedPorts' | 'environment' | 'labels'
> & {
  repoTags: string[]
  repoDigests: string[]
  exposedPorts: string[]
  environment: string[]
  labels: DockerLabel[]
}

export type DockerVolumeDetailView = Omit<DockerVolumeDetail, 'labels' | 'options'> & {
  labels: DockerLabel[]
  options: DockerLabel[]
}

export type DockerNetworkDetailView = Omit<DockerNetworkDetail, 'subnets' | 'containers' | 'labels' | 'options'> & {
  subnets: DockerSubnet[]
  containers: DockerNetworkMember[]
  labels: DockerLabel[]
  options: DockerLabel[]
}

export interface DockerEnvironment {
  available: boolean
  dockerVersion: string
  apiVersion: string
  composeVersion: string
  message: string
}

export interface DockerContainer {
  id: string
  name: string
  image: string
  state: string
  status: string
  ports: string

  project: string

  projectDir: string
  service: string
  createdAt: string
}

export interface DockerPortMapping {
  privatePort: number
  publicPort: number
  type: string
}

export interface DockerMount {
  type: string
  name: string
  source: string
  destination: string
  readOnly: boolean
}

export interface DockerLabel {
  key: string
  value: string
}

export interface DockerContainerDetail {
  id: string
  name: string
  image: string

  entrypoint: string

  command: string
  created: string
  state: string
  status: string
  ports: DockerPortMapping[]
  mounts: DockerMount[]
  environment: string[]
  labels: DockerLabel[]
}

export type DockerContainerAction = 'start' | 'stop' | 'restart' | 'remove'

export type DockerComposeAction = 'up' | 'stop' | 'restart' | 'down'

export interface DockerGateway {
  readonly persistent: boolean

  panelEnabled(sessionId: string): Promise<boolean>

  setPanelEnabled(hostId: string, enabled: boolean): Promise<void>
  environment(sessionId: string): Promise<DockerEnvironment>
  containers(sessionId: string): Promise<DockerContainer[]>
  containerDetail(sessionId: string, containerId: string): Promise<DockerContainerDetail>

  imageDetail(sessionId: string, imageRef: string): Promise<DockerImageDetailView>

  volumeDetail(sessionId: string, volumeName: string): Promise<DockerVolumeDetailView>

  networkDetail(sessionId: string, networkId: string): Promise<DockerNetworkDetailView>

  openContainerLogs(sessionId: string, containerId: string): Promise<void>

  openContainerStats(sessionId: string, containerId: string): Promise<void>

  openComposeLogs(sessionId: string, containerId: string): Promise<void>

  openComposeStats(sessionId: string, containerId: string): Promise<void>

  composeAction(sessionId: string, containerId: string, action: DockerComposeAction): Promise<void>

  composeFilePath(sessionId: string, containerId: string): Promise<string>

  containerAction(sessionId: string, containerId: string, action: DockerContainerAction, force: boolean): Promise<void>

  openContainerTerminal(sessionId: string, containerId: string): Promise<void>

  images(sessionId: string): Promise<DockerImage[]>

  volumes(sessionId: string): Promise<DockerVolume[]>

  networks(sessionId: string): Promise<DockerNetwork[]>

  removeImage(sessionId: string, target: string): Promise<void>

  removeVolume(sessionId: string, volumeName: string): Promise<void>

  removeNetwork(sessionId: string, networkId: string): Promise<void>

  pruneImages(sessionId: string, all: boolean): Promise<void>

  pruneNetworks(sessionId: string): Promise<void>

  pruneVolumes?(sessionId: string): Promise<void>
}

function missingBinding(name: string): AppError {
  return { code: 'UNAVAILABLE', messageKey: 'docker.unavailable', safeDetail: `docker binding missing: ${name}` }
}

class WailsDockerGateway implements DockerGateway {
  readonly persistent = true

  async panelEnabled(sessionId: string): Promise<boolean> {
    const { DockerPanelEnabled } = await import('../../bindings/changeme/internal/app/service')
    return DockerPanelEnabled(sessionId)
  }

  async setPanelEnabled(hostId: string, enabled: boolean): Promise<void> {
    const { SetDockerPanelEnabled } = await import('../../bindings/changeme/internal/app/service')
    await SetDockerPanelEnabled(hostId, enabled)
  }

  async environment(sessionId: string): Promise<DockerEnvironment> {
    const { DockerEnvironment } = await import('../../bindings/changeme/internal/app/service')
    return DockerEnvironment(sessionId)
  }

  async containers(sessionId: string): Promise<DockerContainer[]> {
    const { DockerContainers } = await import('../../bindings/changeme/internal/app/service')

    return (await DockerContainers(sessionId)) ?? []
  }

  async containerDetail(sessionId: string, containerId: string): Promise<DockerContainerDetail> {
    const { DockerContainerDetail } = await import('../../bindings/changeme/internal/app/service')
    const detail = await DockerContainerDetail(sessionId, containerId)

    return {
      ...detail,
      ports: detail.ports ?? [],
      mounts: detail.mounts ?? [],
      environment: detail.environment ?? [],
      labels: detail.labels ?? [],
    }
  }

  async imageDetail(sessionId: string, imageRef: string): Promise<DockerImageDetailView> {
    const { DockerImageDetail } = await import('../../bindings/changeme/internal/app/service')
    const detail = await DockerImageDetail(sessionId, imageRef)

    return {
      ...detail,
      repoTags: detail.repoTags ?? [],
      repoDigests: detail.repoDigests ?? [],
      exposedPorts: detail.exposedPorts ?? [],
      environment: detail.environment ?? [],
      labels: detail.labels ?? [],
    }
  }

  async volumeDetail(sessionId: string, volumeName: string): Promise<DockerVolumeDetailView> {
    const { DockerVolumeDetail } = await import('../../bindings/changeme/internal/app/service')
    const detail = await DockerVolumeDetail(sessionId, volumeName)
    return { ...detail, labels: detail.labels ?? [], options: detail.options ?? [] }
  }

  async networkDetail(sessionId: string, networkId: string): Promise<DockerNetworkDetailView> {
    const { DockerNetworkDetail } = await import('../../bindings/changeme/internal/app/service')
    const detail = await DockerNetworkDetail(sessionId, networkId)
    return {
      ...detail,
      subnets: detail.subnets ?? [],
      containers: detail.containers ?? [],
      labels: detail.labels ?? [],
      options: detail.options ?? [],
    }
  }

  async openContainerLogs(sessionId: string, containerId: string): Promise<void> {
    const { DockerOpenContainerLogs } = await import('../../bindings/changeme/internal/app/service')
    await DockerOpenContainerLogs(sessionId, containerId)
  }

  async openContainerStats(sessionId: string, containerId: string): Promise<void> {
    const { DockerOpenContainerStats } = await import('../../bindings/changeme/internal/app/service')
    await DockerOpenContainerStats(sessionId, containerId)
  }

  async openComposeLogs(sessionId: string, containerId: string): Promise<void> {
    const { DockerOpenComposeLogs } = await import('../../bindings/changeme/internal/app/service')
    await DockerOpenComposeLogs(sessionId, containerId)
  }

  async openComposeStats(sessionId: string, containerId: string): Promise<void> {
    const { DockerOpenComposeStats } = await import('../../bindings/changeme/internal/app/service')
    await DockerOpenComposeStats(sessionId, containerId)
  }

  async composeAction(sessionId: string, containerId: string, action: DockerComposeAction): Promise<void> {
    const { DockerComposeAction } = await import('../../bindings/changeme/internal/app/service')
    await DockerComposeAction(sessionId, containerId, action)
  }

  async composeFilePath(sessionId: string, containerId: string): Promise<string> {
    const { DockerComposeFilePath } = await import('../../bindings/changeme/internal/app/service')
    return (await DockerComposeFilePath(sessionId, containerId)) ?? ''
  }

  async containerAction(
    sessionId: string,
    containerId: string,
    action: DockerContainerAction,
    force: boolean,
  ): Promise<void> {
    const { DockerContainerAction } = await import('../../bindings/changeme/internal/app/service')
    await DockerContainerAction(sessionId, containerId, action, force)
  }

  async openContainerTerminal(sessionId: string, containerId: string): Promise<void> {
    const { DockerOpenContainerTerminal } = await import('../../bindings/changeme/internal/app/service')
    await DockerOpenContainerTerminal(sessionId, containerId)
  }

  async images(sessionId: string): Promise<DockerImage[]> {
    const { DockerImages } = await import('../../bindings/changeme/internal/app/service')

    return (await DockerImages(sessionId)) ?? []
  }

  async volumes(sessionId: string): Promise<DockerVolume[]> {
    const { DockerVolumes } = await import('../../bindings/changeme/internal/app/service')
    return (await DockerVolumes(sessionId)) ?? []
  }

  async networks(sessionId: string): Promise<DockerNetwork[]> {
    const { DockerNetworks } = await import('../../bindings/changeme/internal/app/service')
    return (await DockerNetworks(sessionId)) ?? []
  }

  async removeImage(sessionId: string, target: string): Promise<void> {
    const { DockerRemoveImage } = await import('../../bindings/changeme/internal/app/service')
    await DockerRemoveImage(sessionId, target)
  }

  async removeVolume(sessionId: string, volumeName: string): Promise<void> {
    const { DockerRemoveVolume } = await import('../../bindings/changeme/internal/app/service')
    await DockerRemoveVolume(sessionId, volumeName)
  }

  async removeNetwork(sessionId: string, networkId: string): Promise<void> {
    const { DockerRemoveNetwork } = await import('../../bindings/changeme/internal/app/service')
    await DockerRemoveNetwork(sessionId, networkId)
  }

  async pruneImages(sessionId: string, all: boolean): Promise<void> {
    const { DockerPruneImages } = await import('../../bindings/changeme/internal/app/service')
    await DockerPruneImages(sessionId, all)
  }

  async pruneNetworks(sessionId: string): Promise<void> {
    const { DockerPruneNetworks } = await import('../../bindings/changeme/internal/app/service')
    await DockerPruneNetworks(sessionId)
  }

  async pruneVolumes(sessionId: string): Promise<void> {
    const { DockerPruneVolumes } = await import('../../bindings/changeme/internal/app/service')
    await DockerPruneVolumes(sessionId)
  }
}

class MemoryDockerGateway implements DockerGateway {
  readonly persistent = false

  async panelEnabled(): Promise<boolean> {
    return false
  }

  async setPanelEnabled(): Promise<void> {
    throw missingBinding('SetDockerPanelEnabled')
  }

  async environment(): Promise<DockerEnvironment> {
    return {
      available: false,
      dockerVersion: '',
      apiVersion: '',
      composeVersion: '',
      message: 'docker.unavailable',
    }
  }

  async containers(): Promise<DockerContainer[]> {
    return []
  }

  async images(): Promise<DockerImage[]> {
    return []
  }

  async volumes(): Promise<DockerVolume[]> {
    return []
  }

  async networks(): Promise<DockerNetwork[]> {
    return []
  }

  async containerDetail(): Promise<DockerContainerDetail> {
    throw missingBinding('DockerContainerDetail')
  }

  async imageDetail(): Promise<DockerImageDetailView> {
    throw missingBinding('DockerImageDetail')
  }

  async volumeDetail(): Promise<DockerVolumeDetailView> {
    throw missingBinding('DockerVolumeDetail')
  }

  async networkDetail(): Promise<DockerNetworkDetailView> {
    throw missingBinding('DockerNetworkDetail')
  }

  async openContainerLogs(): Promise<void> {
    throw missingBinding('DockerOpenContainerLogs')
  }

  async openContainerStats(): Promise<void> {
    throw missingBinding('DockerOpenContainerStats')
  }

  async openComposeLogs(): Promise<void> {
    throw missingBinding('DockerOpenComposeLogs')
  }

  async openComposeStats(): Promise<void> {
    throw missingBinding('DockerOpenComposeStats')
  }

  async composeAction(): Promise<void> {
    throw missingBinding('DockerComposeAction')
  }

  async composeFilePath(): Promise<string> {
    throw missingBinding('DockerComposeFilePath')
  }

  async containerAction(): Promise<void> {
    throw missingBinding('DockerContainerAction')
  }

  async openContainerTerminal(): Promise<void> {
    throw missingBinding('DockerOpenContainerTerminal')
  }

  async removeImage(): Promise<void> {
    throw missingBinding('DockerRemoveImage')
  }

  async removeVolume(): Promise<void> {
    throw missingBinding('DockerRemoveVolume')
  }

  async removeNetwork(): Promise<void> {
    throw missingBinding('DockerRemoveNetwork')
  }

  async pruneImages(): Promise<void> {
    throw missingBinding('DockerPruneImages')
  }

  async pruneNetworks(): Promise<void> {
    throw missingBinding('DockerPruneNetworks')
  }

  async pruneVolumes(): Promise<void> {
    throw missingBinding('DockerPruneVolumes')
  }
}

export function createDockerGateway(): DockerGateway {
  return createRuntimeGateway<DockerGateway>(
    () => new WailsDockerGateway(),
    () => new MemoryDockerGateway(),
  )
}

export function isDanglingImage(image: DockerImage): boolean {
  const repository = (image.repository ?? '').trim()
  return repository === '' || repository === '<none>'
}

export function dockerImageName(image: DockerImage): string {
  if (isDanglingImage(image)) return ''
  const repository = (image.repository ?? '').trim()
  const tag = (image.tag ?? '').trim()
  return tag ? `${repository}:${tag}` : repository
}

export function dockerRemoveTarget(image: DockerImage): string {
  const repository = (image.repository ?? '').trim()
  const tag = (image.tag ?? '').trim()
  const tagUsable = tag !== '' && tag !== '<none>'
  if (!isDanglingImage(image) && tagUsable) return `${repository}:${tag}`
  return (image.id ?? '').trim()
}

export function dockerShortId(id: string): string {
  const value = (id ?? '').trim()
  if (!value) return ''
  const digest = value.startsWith('sha256:') ? value.slice('sha256:'.length) : value
  return digest.slice(0, 12)
}

export function dockerUnavailableDescriptionKey(message: string): string {
  const key = (message ?? '').trim().toLowerCase()
  if (key.includes('permission') || key.includes('denied') || key.includes('not in docker group')) {
    return 'shell.docker.unavailablePermission'
  }
  if (key.includes('daemon') || key.includes('cannot connect') || key.includes('is the docker daemon running')) {
    return 'shell.docker.unavailableDaemon'
  }
  return 'shell.docker.unavailableNoCli'
}

export function dockerStateLabelKey(state: string): string {
  switch ((state ?? '').trim().toLowerCase()) {
    case 'running':
      return 'shell.docker.stateRunning'
    case 'exited':
      return 'shell.docker.stateExited'
    case 'created':
      return 'shell.docker.stateCreated'
    case 'paused':
      return 'shell.docker.statePaused'
    case 'restarting':
      return 'shell.docker.stateRestarting'
    case 'dead':
      return 'shell.docker.stateDead'
    default:
      return ''
  }
}
