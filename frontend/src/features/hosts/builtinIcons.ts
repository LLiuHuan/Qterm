import aix from '../../assets/logos/aix.svg'
import alibabaCloudLinux from '../../assets/logos/alibaba-cloud-linux.svg'
import almaLinux from '../../assets/logos/alma-linux.svg'
import anolis from '../../assets/logos/anolis.svg'
import alpineLinux from '../../assets/logos/alpine-linux.svg'
import android from '../../assets/logos/android.svg'
import amazonLinux from '../../assets/logos/amazon-linux.svg'
import apple from '../../assets/logos/apple.svg'
import archLinux from '../../assets/logos/arch-linux.svg'
import aws from '../../assets/logos/aws.svg'
import centos from '../../assets/logos/centos.svg'
import debian from '../../assets/logos/debian.svg'
import deepin from '../../assets/logos/deepin.svg'
import dragonflyBSD from '../../assets/logos/dragonfly-bsd.svg'
import eulerOS from '../../assets/logos/euleros.svg'
import elementary from '../../assets/logos/elementary.svg'
import fedora from '../../assets/logos/fedora.svg'
import freebsd from '../../assets/logos/freebsd.svg'
import gentoo from '../../assets/logos/gentoo.svg'
import googleChrome from '../../assets/logos/google-chrome.svg'
import harmonyOS from '../../assets/logos/HarmonyOS.svg'
import illumos from '../../assets/logos/illumos.svg'
import kaliLinux from '../../assets/logos/kali-linux.svg'
import kylin from '../../assets/logos/kylin.svg'
import linux from '../../assets/logos/linux.svg'
import linuxMint from '../../assets/logos/linux-mint.svg'
import manjaro from '../../assets/logos/manjaro.svg'
import netbsd from '../../assets/logos/netbsd.svg'
import nixos from '../../assets/logos/nixos.svg'
import noVnc from '../../assets/logos/no-vnc.svg'
import openbsd from '../../assets/logos/openbsd.svg'
import opensuse from '../../assets/logos/opensuse.svg'
import openwrt from '../../assets/logos/openwrt.svg'
import oracleLinux from '../../assets/logos/oracle-linux.svg'
import openCloudOS from '../../assets/logos/OpenCloudOS.svg'
import openEuler from '../../assets/logos/openEuler.svg'
import parrotSecurity from '../../assets/logos/parrot-security.svg'
import popOS from '../../assets/logos/pop-os.svg'
import raspberryPi from '../../assets/logos/raspberry-pi.svg'
import redHat from '../../assets/logos/red-hat.svg'
import rocky from '../../assets/logos/rocky-linux.svg'
import rhel from '../../assets/logos/rhel.svg'
import steamOS from '../../assets/logos/steamos.svg'
import tencentOS from '../../assets/logos/TencentOS.svg'
import tigerVnc from '../../assets/logos/tiger-vnc.svg'
import ubuntu from '../../assets/logos/ubuntu.svg'
import ultralVnc from '../../assets/logos/ultral-vnc.svg'
import unix from '../../assets/logos/unix.svg'
import uos from '../../assets/logos/uos.svg'
import voidLinux from '../../assets/logos/void-linux.svg'
import windows from '../../assets/logos/windows.svg'
import windows10 from '../../assets/logos/windows-10.svg'
import windows11 from '../../assets/logos/windows-11.svg'
import windows7 from '../../assets/logos/windows-7.svg'

const linuxFallback = linux

export type BuiltinHostIcon = {
  value: string
  label: string
  src: string
  themeAware?: boolean
}

export const builtinHostIcons: readonly BuiltinHostIcon[] = [
  { value: 'android', label: 'Android', src: android },
  { value: 'macos', label: 'macOS', src: apple, themeAware: true },
  { value: 'windows', label: 'Windows', src: windows },
  { value: 'linux', label: 'Linux', src: linux },
  { value: 'linux-mint', label: 'Linux Mint', src: linuxMint },
  { value: 'ubuntu', label: 'Ubuntu', src: ubuntu },
  { value: 'debian', label: 'Debian', src: debian },
  { value: 'fedora', label: 'Fedora', src: fedora },
  { value: 'centos', label: 'CentOS', src: centos },
  { value: 'rocky-linux', label: 'Rocky Linux', src: rocky },
  { value: 'alma-linux', label: 'AlmaLinux', src: almaLinux },
  { value: 'alpine-linux', label: 'Alpine Linux', src: alpineLinux },
  { value: 'amazon-linux', label: 'Amazon Linux', src: amazonLinux },
  { value: 'arch-linux', label: 'Arch Linux', src: archLinux },
  { value: 'gentoo', label: 'Gentoo', src: gentoo },
  { value: 'kali-linux', label: 'Kali Linux', src: kaliLinux },
  { value: 'manjaro', label: 'Manjaro', src: manjaro },
  { value: 'nixos', label: 'NixOS', src: nixos },
  { value: 'opensuse', label: 'openSUSE', src: opensuse },
  { value: 'openwrt', label: 'OpenWrt', src: openwrt },
  { value: 'oracle-linux', label: 'Oracle Linux', src: oracleLinux },
  { value: 'pop-os', label: 'Pop!_OS', src: popOS, themeAware: true },
  { value: 'raspberry-pi', label: 'Raspberry Pi OS', src: raspberryPi },
  { value: 'parrot-security', label: 'Parrot OS', src: parrotSecurity },
  { value: 'void-linux', label: 'Void Linux', src: voidLinux },
  { value: 'elementary', label: 'elementary OS', src: elementary },
  { value: 'deepin', label: 'Deepin', src: deepin },
  { value: 'freebsd', label: 'FreeBSD', src: freebsd },
  { value: 'openbsd', label: 'OpenBSD', src: openbsd },
  { value: 'netbsd', label: 'NetBSD', src: netbsd },
  { value: 'dragonfly-bsd', label: 'DragonFly BSD', src: dragonflyBSD, themeAware: true },
  { value: 'bsd', label: 'BSD', src: linuxFallback },
  { value: 'illumos', label: 'illumos', src: illumos, themeAware: true },
  { value: 'aix', label: 'AIX', src: aix },
  { value: 'unix', label: 'Unix', src: unix },
  { value: 'chromeos', label: 'ChromeOS', src: linuxFallback },
  { value: 'google-chrome', label: 'Google Chrome', src: googleChrome },
  { value: 'red-hat', label: 'Red Hat', src: redHat },
  { value: 'rhel', label: 'Red Hat Enterprise Linux', src: rhel },
  { value: 'steamos', label: 'SteamOS', src: steamOS },
  { value: 'aws', label: 'AWS', src: aws },
  { value: 'windows-11', label: 'Windows 11', src: windows11 },
  { value: 'windows-10', label: 'Windows 10', src: windows10 },
  { value: 'windows-7', label: 'Windows 7', src: windows7 },

  { value: 'uos', label: '统信 UOS', src: uos },
  { value: 'kylin', label: '银河麒麟', src: kylin },
  { value: 'openkylin', label: '开放麒麟 openKylin', src: linuxFallback },
  { value: 'neokylin', label: '中标麒麟 NeoKylin', src: linuxFallback },
  { value: 'openeuler', label: 'openEuler', src: openEuler },
  { value: 'euleros', label: 'EulerOS', src: eulerOS, themeAware: true },
  { value: 'anolis', label: 'Anolis OS', src: anolis },
  { value: 'loongnix', label: 'Loongnix', src: linuxFallback },
  { value: 'tencentos', label: 'TencentOS Server', src: tencentOS },
  { value: 'alinux', label: 'Alibaba Cloud Linux', src: alibabaCloudLinux },
  { value: 'opencloudos', label: 'OpenCloudOS', src: openCloudOS },
  { value: 'harmonyos', label: 'HarmonyOS', src: harmonyOS },

  { value: 'no-vnc', label: 'noVNC', src: noVnc },
  { value: 'tiger-vnc', label: 'TigerVNC', src: tigerVnc },
  { value: 'ultral-vnc', label: 'UltraVNC', src: ultralVnc },
]

export const hostIconPickerOptions = builtinHostIcons.filter(
  (item) => item.value === 'linux' || item.src !== linuxFallback,
)
