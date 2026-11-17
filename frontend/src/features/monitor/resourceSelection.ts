import type { ResourceSnapshot } from '../../services/sessions'

type NetworkResource = ResourceSnapshot['networks'][number]
type DiskResource = ResourceSnapshot['disks'][number]

export function resourceNetworks(
  networks: ResourceSnapshot['networks'] | null | undefined,
  expanded = false,
): ResourceSnapshot['networks'] {
  const sorted = [...(networks ?? [])].filter((item) => item && item.name.trim()).sort(compareNetworks)
  if (expanded) return sorted

  const hostInterfaces = sorted.filter((item) => !isLoopback(item.name) && !isVirtualInterface(item.name))
  if (hostInterfaces.length > 0) return [hostInterfaces[0]]

  const nonLoopback = sorted.filter((item) => !isLoopback(item.name))
  return nonLoopback.length > 0 ? [nonLoopback[0]] : sorted.slice(0, 1)
}

export function resourceDisks(
  disks: ResourceSnapshot['disks'] | null | undefined,
  expanded = false,
): ResourceSnapshot['disks'] {
  const sorted = [...(disks ?? [])].filter((item) => item && item.mount.trim()).sort(compareDisks)
  if (expanded) return sorted

  return sorted.filter((item) => {
    if (item.mount === '/') return true
    if (isPseudoMount(item.mount)) return false
    return isPersistentFileSystem(item.fileSystemType) ? true : !isKnownEphemeralFileSystem(item.fileSystemType)
  })
}

function compareNetworks(left: NetworkResource, right: NetworkResource): number {
  const rank = networkRank(left.name) - networkRank(right.name)
  return rank || left.name.localeCompare(right.name)
}

function networkRank(name: string): number {
  const normalized = name.trim().toLowerCase()
  if (isLoopback(normalized)) return 1000
  if (isVirtualInterface(normalized)) return 900

  if (/^(bond|team)[0-9a-z._-]*$/.test(normalized)) return 0
  if (/^ens[0-9a-z._-]*$/.test(normalized)) return 10
  if (/^(eno|enp|enx)[0-9a-z._-]*$/.test(normalized)) return 11
  if (/^eth[0-9a-z._-]*$/.test(normalized)) return 12
  if (/^(wlan|wlp)[0-9a-z._-]*$/.test(normalized)) return 13
  return 100
}

function isLoopback(name: string): boolean {
  return name.trim().toLowerCase() === 'lo'
}

function isVirtualInterface(name: string): boolean {
  const normalized = name.trim().toLowerCase()
  return /^(veth|docker|cni|flannel|virbr|br-|cali|tun|tap|wg|tailscale|vxlan|dummy|podman|kube|sit|gre|gretap)/.test(
    normalized,
  )
}

function compareDisks(left: DiskResource, right: DiskResource): number {
  const rootRank = Number(right.mount === '/') - Number(left.mount === '/')
  return rootRank || left.mount.localeCompare(right.mount) || left.fileSystemType.localeCompare(right.fileSystemType)
}

function isPersistentFileSystem(fileSystemType: string): boolean {
  return /^(ext[234]|xfs|btrfs|zfs|f2fs|vfat|exfat|ntfs|ntfs3|fuseblk|ufs|apfs|hfs|hfsplus)$/i.test(
    fileSystemType.trim(),
  )
}

function isKnownEphemeralFileSystem(fileSystemType: string): boolean {
  return /^(tmpfs|devtmpfs|proc|procfs|sysfs|cgroup2?|squashfs|ramfs|pstore|debugfs|tracefs|securityfs|configfs|fusectl|overlay|juicefs|s3fs|ossfs2?|nfs|nfs4|cifs|smb3|ceph|lustre|glusterfs|9p|fuse\.[a-z0-9_-]+)$/i.test(
    fileSystemType.trim(),
  )
}

function isPseudoMount(mount: string): boolean {
  const normalized = mount.trim().toLowerCase()
  return (
    normalized === '/dev' ||
    normalized.startsWith('/dev/') ||
    normalized === '/proc' ||
    normalized.startsWith('/proc/') ||
    normalized === '/sys' ||
    normalized.startsWith('/sys/') ||
    normalized === '/run' ||
    normalized.startsWith('/run/')
  )
}
