import type { HostRecord } from '@/services/hosts.ts'
import type { SessionTab } from '../../types/qterm'
import { isCustomHostIcon } from './customHostIcon'

export type AutomaticHostIcon =
  | 'android'
  | 'macos'
  | 'windows'
  | 'windows-11'
  | 'windows-10'
  | 'windows-7'
  | 'aws'
  | 'google-chrome'
  | 'linux'
  | 'linux-mint'
  | 'ubuntu'
  | 'debian'
  | 'fedora'
  | 'centos'
  | 'rocky-linux'
  | 'alma-linux'
  | 'alpine-linux'
  | 'amazon-linux'
  | 'arch-linux'
  | 'gentoo'
  | 'kali-linux'
  | 'manjaro'
  | 'nixos'
  | 'opensuse'
  | 'openwrt'
  | 'oracle-linux'
  | 'pop-os'
  | 'raspberry-pi'
  | 'parrot-security'
  | 'void-linux'
  | 'elementary'
  | 'deepin'
  | 'freebsd'
  | 'openbsd'
  | 'netbsd'
  | 'dragonfly-bsd'
  | 'illumos'
  | 'aix'
  | 'chromeos'
  | 'red-hat'
  | 'rhel'
  | 'steamos'
  | 'uos'
  | 'kylin'
  | 'openkylin'
  | 'neokylin'
  | 'openeuler'
  | 'euleros'
  | 'anolis'
  | 'loongnix'
  | 'tencentos'
  | 'alinux'
  | 'opencloudos'
  | 'harmonyos'
  | 'no-vnc'
  | 'tiger-vnc'
  | 'ultral-vnc'
  | 'bsd'
  | 'unix'

export const automaticHostIconGlyphs: Readonly<Record<AutomaticHostIcon, string>> = {
  android: 'A',
  macos: '⌘',
  windows: '⊞',
  'windows-11': '⊞',
  'windows-10': '⊞',
  'windows-7': '⊞',
  aws: 'A',
  'google-chrome': 'C',
  linux: 'L',
  'linux-mint': 'M',
  ubuntu: 'U',
  debian: 'D',
  fedora: 'F',
  centos: 'C',
  'rocky-linux': 'R',
  'alma-linux': 'A',
  'alpine-linux': 'A',
  'amazon-linux': 'A',
  'arch-linux': 'A',
  gentoo: 'G',
  'kali-linux': 'K',
  manjaro: 'M',
  nixos: 'N',
  opensuse: 'S',
  openwrt: 'W',
  'oracle-linux': 'O',
  'pop-os': 'P',
  'raspberry-pi': 'R',
  'parrot-security': 'P',
  'void-linux': 'V',
  elementary: 'E',
  deepin: 'D',
  freebsd: 'F',
  openbsd: 'O',
  netbsd: 'N',
  'dragonfly-bsd': 'D',
  illumos: 'I',
  aix: 'A',
  chromeos: 'C',
  'red-hat': 'R',
  rhel: 'R',
  steamos: 'S',
  uos: '统',
  kylin: '麒',
  openkylin: '开',
  neokylin: '中',
  openeuler: 'E',
  euleros: 'E',
  anolis: 'A',
  loongnix: 'L',
  tencentos: 'T',
  alinux: 'A',
  opencloudos: 'O',
  harmonyos: '鸿',
  'no-vnc': 'V',
  'tiger-vnc': 'T',
  'ultral-vnc': 'U',
  bsd: 'B',
  unix: 'U',
}

export const automaticHostIconTitles: Readonly<Record<AutomaticHostIcon, string>> = {
  android: 'Android',
  macos: 'macOS',
  windows: 'Windows',
  'windows-11': 'Windows 11',
  'windows-10': 'Windows 10',
  'windows-7': 'Windows 7',
  aws: 'AWS',
  'google-chrome': 'Google Chrome',
  linux: 'Linux',
  'linux-mint': 'Linux Mint',
  ubuntu: 'Ubuntu',
  debian: 'Debian',
  fedora: 'Fedora',
  centos: 'CentOS',
  'rocky-linux': 'Rocky Linux',
  'alma-linux': 'AlmaLinux',
  'alpine-linux': 'Alpine Linux',
  'amazon-linux': 'Amazon Linux',
  'arch-linux': 'Arch Linux',
  gentoo: 'Gentoo',
  'kali-linux': 'Kali Linux',
  manjaro: 'Manjaro',
  nixos: 'NixOS',
  opensuse: 'openSUSE',
  openwrt: 'OpenWrt',
  'oracle-linux': 'Oracle Linux',
  'pop-os': 'Pop!_OS',
  'raspberry-pi': 'Raspberry Pi OS',
  'parrot-security': 'Parrot OS',
  'void-linux': 'Void Linux',
  elementary: 'elementary OS',
  deepin: 'Deepin',
  freebsd: 'FreeBSD',
  openbsd: 'OpenBSD',
  netbsd: 'NetBSD',
  'dragonfly-bsd': 'DragonFly BSD',
  illumos: 'illumos',
  aix: 'AIX',
  chromeos: 'ChromeOS',
  'red-hat': 'Red Hat',
  rhel: 'Red Hat Enterprise Linux',
  steamos: 'SteamOS',
  uos: '统信 UOS',
  kylin: '银河麒麟',
  openkylin: '开放麒麟 openKylin',
  neokylin: '中标麒麟 NeoKylin',
  openeuler: 'openEuler',
  euleros: 'EulerOS',
  anolis: 'Anolis OS',
  loongnix: 'Loongnix',
  tencentos: 'TencentOS Server',
  alinux: 'Alibaba Cloud Linux',
  opencloudos: 'OpenCloudOS',
  harmonyos: 'HarmonyOS',
  'no-vnc': 'noVNC',
  'tiger-vnc': 'TigerVNC',
  'ultral-vnc': 'UltraVNC',
  bsd: 'BSD',
  unix: 'Unix',
}

type HostIconSource = Pick<HostRecord, 'icon' | 'name' | 'protocol'>
type SessionHostIconSource = HostIconSource & Pick<HostRecord, 'id'>

export const DEFAULT_VNC_HOST_ICON = 'no-vnc'

export function resolveHostIcon(host: Pick<HostRecord, 'icon' | 'protocol'>): string {
  const icon = (host.icon ?? '').trim()
  if (icon) return icon
  return host.protocol === 'vnc' ? DEFAULT_VNC_HOST_ICON : ''
}

export function hostIconLabel(host: HostIconSource): string {
  const icon = resolveHostIcon(host)
  if (isCustomHostIcon(icon) || icon.startsWith('data:')) return '自定义图标'
  const automaticIcon = automaticHostIconGlyphs[icon.toLocaleLowerCase() as AutomaticHostIcon]
  return automaticIcon || icon || host.name.slice(0, 1).toLocaleUpperCase()
}

export function hostIconTitle(host: HostIconSource): string {
  const icon = resolveHostIcon(host)
  if (isCustomHostIcon(icon) || icon.startsWith('data:')) return '自定义图标'
  const automaticTitle = automaticHostIconTitles[icon.toLocaleLowerCase() as AutomaticHostIcon]
  return automaticTitle || (icon ? '自定义图标' : '自动识别')
}

export function sessionIconLabel(session: SessionTab, hosts: readonly SessionHostIconSource[]): string {
  const host = hosts.find((item) => item.id === session.hostId)
  return host ? hostIconLabel(host) : session.title.slice(0, 1).toLocaleUpperCase()
}
