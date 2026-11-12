import aix from '../../assets/logos/aix.svg'
import alibabaCloudLinux from '../../assets/logos/alibaba-cloud-linux.svg'
import aliyun from '../../assets/logos/阿里云.svg'
import almaLinux from '../../assets/logos/alma-linux.svg'
import alpineLinux from '../../assets/logos/alpine-linux.svg'
import amigaos from '../../assets/logos/amigaos.svg'
import anolis from '../../assets/logos/anolis.svg'
import amazonLinux from '../../assets/logos/amazon-linux.svg'
import android from '../../assets/logos/android.svg'
import apple from '../../assets/logos/apple.svg'
import archLinux from '../../assets/logos/arch-linux.svg'
import armbian from '../../assets/logos/armbian.svg'
import asus from '../../assets/logos/asus.svg'
import aws from '../../assets/logos/aws.svg'
import centos from '../../assets/logos/centos.svg'
import chinaMobile from '../../assets/logos/中国移动.svg'
import chinaTelecom from '../../assets/logos/中国电信.svg'
import chinaUnicom from '../../assets/logos/中国联通.svg'
import cloudcone from '../../assets/logos/cloudcone.svg'
import cloudflare from '../../assets/logos/cloudflare.svg'
import ctyun from '../../assets/logos/ctyun.svg'
import debian from '../../assets/logos/debian.svg'
import deepin from '../../assets/logos/deepin.svg'
import dragonflyBSD from '../../assets/logos/dragonfly-bsd.svg'
import elementary from '../../assets/logos/elementary.svg'
import emby from '../../assets/logos/emby.svg'
import eulerOS from '../../assets/logos/euleros.svg'
import fedora from '../../assets/logos/fedora.svg'
import fnos from '../../assets/logos/fnos.svg'
import freebsd from '../../assets/logos/freebsd.svg'
import freeRTOS from '../../assets/logos/free-rtos.svg'
import fuchsia from '../../assets/logos/fuchsia.svg'
import gcore from '../../assets/logos/gcore.svg'
import gentoo from '../../assets/logos/gentoo.svg'
import googleChrome from '../../assets/logos/google-chrome.svg'
import googleCloud from '../../assets/logos/谷歌云.svg'
import grafana from '../../assets/logos/grafana.svg'
import haiku from '../../assets/logos/haiku.svg'
import harmonyOS from '../../assets/logos/harmonyos.svg'
import ibm from '../../assets/logos/ibm.svg'
import iKuai from '../../assets/logos/爱快.svg'
import illumos from '../../assets/logos/illumos.svg'
import jiKongJian from '../../assets/logos/极空间.svg'
import kaios from '../../assets/logos/kaios.svg'
import kaliLinux from '../../assets/logos/kali-linux.svg'
import kubernetes from '../../assets/logos/kubernetes.svg'
import kylin from '../../assets/logos/kylin.svg'
import linux from '../../assets/logos/linux.svg'
import linuxMint from '../../assets/logos/linux-mint.svg'
import manjaro from '../../assets/logos/manjaro.svg'
import mikrotik from '../../assets/logos/mikrotik.svg'
import neokylin from '../../assets/logos/neokylin.svg'
import netbsd from '../../assets/logos/netbsd.svg'
import nixos from '../../assets/logos/nixos.svg'
import noVnc from '../../assets/logos/no-vnc.svg'
import onedrive from '../../assets/logos/microsoft-onedrive.svg'
import openbsd from '../../assets/logos/openbsd.svg'
import openCloudOS from '../../assets/logos/opencloudos.svg'
import openEuler from '../../assets/logos/openeuler.svg'
import opensuse from '../../assets/logos/opensuse.svg'
import openstack from '../../assets/logos/openstack.svg'
import openwrt from '../../assets/logos/openwrt.svg'
import opnsense from '../../assets/logos/opnsense.svg'
import oracleLinux from '../../assets/logos/oracle-linux.svg'
import orangePi from '../../assets/logos/orange-pi.svg'
import parrotSecurity from '../../assets/logos/parrot-security.svg'
import pfsense from '../../assets/logos/pfsense.svg'
import popOS from '../../assets/logos/pop-os.svg'
import proxmox from '../../assets/logos/proxmox.svg'
import qnap from '../../assets/logos/qnap.svg'
import raspberryPi from '../../assets/logos/raspberry-pi.svg'
import reactos from '../../assets/logos/reactos.svg'
import redHat from '../../assets/logos/red-hat.svg'
import rhel from '../../assets/logos/rhel.svg'
import rocky from '../../assets/logos/rocky-linux.svg'
import slackware from '../../assets/logos/slackware.svg'
import smartisan from '../../assets/logos/smartisan.svg'
import steamOS from '../../assets/logos/steamos.svg'
import synologyDsm from '../../assets/logos/synology-dsm.svg'
import tencentOS from '../../assets/logos/tencentos.svg'
import tencentCloud from '../../assets/logos/腾讯云.svg'
import telsin from '../../assets/logos/浪潮云.svg'
import tigerVnc from '../../assets/logos/tiger-vnc.svg'
import truenas from '../../assets/logos/truenas.svg'
import truenasCore from '../../assets/logos/truenas-core.svg'
import ubuntu from '../../assets/logos/ubuntu.svg'
import ucloud from '../../assets/logos/ucloud.svg'
import ugreen from '../../assets/logos/绿联.svg'
import ultralVnc from '../../assets/logos/ultral-vnc.svg'
import unix from '../../assets/logos/unix.svg'
import unraid from '../../assets/logos/unraid.svg'
import uos from '../../assets/logos/uos.svg'
import virtualbox from '../../assets/logos/virtualbox-2010.svg'
import vmwareWorkstation from '../../assets/logos/vmware.svg'
import voidLinux from '../../assets/logos/void-linux.svg'
import vultr from '../../assets/logos/vultr.svg'
import win7 from '../../assets/logos/win7.svg'
import win10 from '../../assets/logos/win10.svg'
import win11 from '../../assets/logos/win11.svg'
import windows from '../../assets/logos/windows.svg'
import zenlayer from '../../assets/logos/zenlayer.svg'
import zephyr from '../../assets/logos/zephyr.svg'

const linuxFallback = linux

export type BuiltinHostIcon = {
  value: string
  label: string
  src: string
  themeAware?: boolean
}

export const builtinHostIcons: readonly BuiltinHostIcon[] = [
  { value: 'aix', label: 'AIX', src: aix },
  { value: 'alinux', label: 'Alibaba Cloud Linux', src: alibabaCloudLinux },
  { value: 'aliyun', label: '阿里云', src: aliyun },
  { value: 'alma-linux', label: 'AlmaLinux', src: almaLinux },
  { value: 'alpine-linux', label: 'Alpine Linux', src: alpineLinux },
  { value: 'amazon-linux', label: 'Amazon Linux', src: amazonLinux },
  { value: 'amigaos', label: 'AmigaOS', src: amigaos },
  { value: 'android', label: 'Android', src: android },
  { value: 'anolis', label: 'Anolis OS', src: anolis },
  { value: 'arch-linux', label: 'Arch Linux', src: archLinux },

  { value: 'armbian', label: 'Armbian', src: armbian },
  { value: 'asus', label: '华硕 ASUS', src: asus },
  { value: 'aws', label: 'AWS', src: aws },
  { value: 'bsd', label: 'BSD', src: linuxFallback },
  { value: 'centos', label: 'CentOS', src: centos },
  { value: 'china-mobile', label: '中国移动', src: chinaMobile },
  { value: 'china-telecom', label: '中国电信', src: chinaTelecom },
  { value: 'china-unicom', label: '中国联通', src: chinaUnicom },
  { value: 'chromeos', label: 'ChromeOS', src: linuxFallback },
  { value: 'cloudcone', label: 'CloudCone', src: cloudcone },
  { value: 'cloudflare', label: 'Cloudflare', src: cloudflare },
  { value: 'ctyun', label: '天翼云', src: ctyun },
  { value: 'debian', label: 'Debian', src: debian },
  { value: 'deepin', label: 'Deepin', src: deepin },
  { value: 'dragonfly-bsd', label: 'DragonFly BSD', src: dragonflyBSD, themeAware: true },
  { value: 'elementary', label: 'elementary OS', src: elementary },
  { value: 'emby', label: 'Emby', src: emby },
  { value: 'euleros', label: 'EulerOS', src: eulerOS, themeAware: true },
  { value: 'fedora', label: 'Fedora', src: fedora },
  { value: 'fnos', label: '飞牛 fnOS', src: fnos },
  { value: 'freebsd', label: 'FreeBSD', src: freebsd },
  { value: 'freertos', label: 'FreeRTOS', src: freeRTOS },
  { value: 'fuchsia', label: 'Fuchsia', src: fuchsia },
  { value: 'gcore', label: 'Gcore', src: gcore },
  { value: 'gentoo', label: 'Gentoo', src: gentoo },
  { value: 'google-chrome', label: 'Google Chrome', src: googleChrome },
  { value: 'google-cloud', label: '谷歌云 Google Cloud', src: googleCloud },
  { value: 'grafana', label: 'Grafana', src: grafana },
  { value: 'haiku', label: 'Haiku', src: haiku },
  { value: 'harmonyos', label: 'HarmonyOS', src: harmonyOS },
  { value: 'ibm', label: 'IBM', src: ibm },
  { value: 'ikuai', label: '爱快 iKuai', src: iKuai },
  { value: 'illumos', label: 'illumos', src: illumos, themeAware: true },
  { value: 'inspur-cloud', label: '浪潮云', src: telsin },
  { value: 'kaios', label: 'KaiOS', src: kaios },
  { value: 'kali-linux', label: 'Kali Linux', src: kaliLinux },
  { value: 'kubernetes', label: 'Kubernetes', src: kubernetes },
  { value: 'kylin', label: '银河麒麟', src: kylin },
  { value: 'linux', label: 'Linux', src: linux },
  { value: 'linux-mint', label: 'Linux Mint', src: linuxMint },
  { value: 'loongnix', label: 'Loongnix', src: linuxFallback },
  { value: 'macos', label: 'macOS', src: apple, themeAware: true },
  { value: 'manjaro', label: 'Manjaro', src: manjaro },
  { value: 'mikrotik', label: 'MikroTik RouterOS', src: mikrotik, themeAware: true },
  { value: 'neokylin', label: '中标麒麟 NeoKylin', src: neokylin },
  { value: 'netbsd', label: 'NetBSD', src: netbsd },
  { value: 'nixos', label: 'NixOS', src: nixos },
  { value: 'no-vnc', label: 'noVNC', src: noVnc },
  { value: 'onedrive', label: 'Microsoft OneDrive', src: onedrive },
  { value: 'openbsd', label: 'OpenBSD', src: openbsd, themeAware: true },
  { value: 'opencloudos', label: 'OpenCloudOS', src: openCloudOS },
  { value: 'openeuler', label: 'openEuler', src: openEuler },
  { value: 'openkylin', label: '开放麒麟 openKylin', src: linuxFallback },
  { value: 'openstack', label: 'OpenStack', src: openstack },
  { value: 'opensuse', label: 'openSUSE', src: opensuse },
  { value: 'openwrt', label: 'OpenWrt', src: openwrt },
  { value: 'opnsense', label: 'OPNsense', src: opnsense },
  { value: 'oracle-linux', label: 'Oracle Linux', src: oracleLinux },
  { value: 'orange-pi', label: '香橙派 Orange Pi', src: orangePi },
  { value: 'parrot-security', label: 'Parrot OS', src: parrotSecurity },
  { value: 'pfsense', label: 'pfSense', src: pfsense, themeAware: true },
  { value: 'pop-os', label: 'Pop!_OS', src: popOS, themeAware: true },
  { value: 'proxmox', label: 'Proxmox VE', src: proxmox },
  { value: 'qnap', label: 'QNAP QTS', src: qnap },
  { value: 'raspberry-pi', label: 'Raspberry Pi OS', src: raspberryPi },
  { value: 'reactos', label: 'ReactOS', src: reactos },
  { value: 'red-hat', label: 'Red Hat', src: redHat },
  { value: 'rhel', label: 'Red Hat Enterprise Linux', src: rhel },
  { value: 'rocky-linux', label: 'Rocky Linux', src: rocky },
  { value: 'slackware', label: 'Slackware', src: slackware, themeAware: true },
  { value: 'smartisan', label: 'Smartisan OS', src: smartisan },
  { value: 'steamos', label: 'SteamOS', src: steamOS },
  { value: 'synology', label: '群晖 DSM', src: synologyDsm },
  { value: 'tencent-cloud', label: '腾讯云', src: tencentCloud },
  { value: 'tencentos', label: 'TencentOS Server', src: tencentOS },
  { value: 'tiger-vnc', label: 'TigerVNC', src: tigerVnc },
  { value: 'truenas', label: 'TrueNAS', src: truenas },
  { value: 'truenas-core', label: 'TrueNAS CORE', src: truenasCore },
  { value: 'ubuntu', label: 'Ubuntu', src: ubuntu },
  { value: 'ucloud', label: 'UCloud 优刻得', src: ucloud },
  { value: 'ugreen-nas', label: '绿联 NAS', src: ugreen },
  { value: 'ultral-vnc', label: 'UltraVNC', src: ultralVnc },
  { value: 'unix', label: 'Unix', src: unix },
  { value: 'unraid', label: 'Unraid', src: unraid },
  { value: 'uos', label: '统信 UOS', src: uos },
  { value: 'virtualbox', label: 'VirtualBox', src: virtualbox },
  { value: 'vmware-workstation', label: 'VMware Workstation', src: vmwareWorkstation },
  { value: 'void-linux', label: 'Void Linux', src: voidLinux },
  { value: 'vultr', label: 'Vultr', src: vultr },
  { value: 'windows', label: 'Windows', src: windows },
  { value: 'windows-10', label: 'Windows 10', src: win10 },
  { value: 'windows-11', label: 'Windows 11', src: win11 },
  { value: 'windows-7', label: 'Windows 7', src: win7 },
  { value: 'zenlayer', label: 'Zenlayer', src: zenlayer },
  { value: 'zephyr', label: 'Zephyr', src: zephyr, themeAware: true },
  { value: 'zspace', label: '极空间 ZSpace', src: jiKongJian },
]

export const hostIconPickerOptions = builtinHostIcons.filter(
  (item) => item.value === 'linux' || item.src !== linuxFallback,
)

export function resolveBuiltinHostIcon(value: string) {
  const direct = builtinHostIcons.find((item) => item.value === value)
  if (direct) return direct
  if (/^macos-\d{1,2}$/.test(value)) {
    return builtinHostIcons.find((item) => item.value === 'macos')
  }
  return undefined
}
