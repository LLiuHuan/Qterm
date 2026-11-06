#!/usr/bin/env python3
"""把 FreeRDP helper 依赖的动态库"随包化"。

为什么需要：helper 里的 rpath 指向**构建机**的绝对路径（.cache/freerdp-build-make/...），
换一台机器或打包发布后 dyld 就会失败。本脚本把用到的库拷到 helper 同级目录，
把所有引用改成 @loader_path/...，最后重新做 ad-hoc 签名
（Apple Silicon 上 install_name_tool 会让原签名失效，不重签无法加载）。

用法：python3 tools/rdp-helper-poc/scripts/relocate-dylibs.py [helper 路径]
"""

from __future__ import annotations

import os
import shutil
import subprocess
import sys

TARGET_LIBRARY_MARKERS = ("libfreerdp", "libwinpr")


def run(*args: str, check: bool = True) -> subprocess.CompletedProcess[str]:
    return subprocess.run(args, capture_output=True, text=True, check=check)


def rpaths(binary: str) -> list[str]:
    """读取二进制的 LC_RPATH 列表。"""
    result: list[str] = []
    lines = run("otool", "-l", binary).stdout.splitlines()
    for index, line in enumerate(lines):
        if "LC_RPATH" in line:
            for follow in lines[index : index + 4]:
                stripped = follow.strip()
                if stripped.startswith("path "):
                    result.append(stripped.split(" ", 1)[1].split(" (offset")[0])
                    break
    return result


def dependencies(binary: str) -> list[str]:
    """列出直接依赖（去掉第一行的自身路径）。"""
    return [
        line.strip().split(" ")[0]
        for line in run("otool", "-L", binary).stdout.splitlines()[1:]
        if line.strip()
    ]


def is_target(dep: str) -> bool:
    return any(marker in dep for marker in TARGET_LIBRARY_MARKERS)


def resolve(dep: str, from_binary: str) -> str | None:
    """把 @rpath/xxx 解析成真实文件路径；绝对路径直接返回。"""
    name = os.path.basename(dep)
    if dep.startswith("/"):
        return dep if os.path.exists(dep) else None
    for root in [os.path.dirname(from_binary), *rpaths(from_binary)]:
        candidate = os.path.join(root, name)
        if os.path.exists(candidate):
            return candidate
    return None


def collect(helper: str) -> dict[str, str]:
    """递归收集 helper 需要的 FreeRDP/WinPR 动态库：{库名: 真实路径}。"""
    found: dict[str, str] = {}
    queue = [helper]
    seen: set[str] = set()
    while queue:
        current = queue.pop()
        if current in seen or not os.path.exists(current):
            continue
        seen.add(current)
        for dep in dependencies(current):
            if not is_target(dep):
                continue
            real = resolve(dep, current)
            if real is None:
                raise SystemExit(f"无法解析依赖真实路径: {dep}（来自 {current}）")
            name = os.path.basename(real)
            found[name] = os.path.realpath(real)
            queue.append(real)
    return found


def main() -> int:
    default = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "freerdp", "build", "qterm-freerdp-helper")
    helper = os.path.abspath(sys.argv[1] if len(sys.argv) > 1 else default)
    if not os.path.isfile(helper):
        raise SystemExit(f"找不到 helper: {helper}")
    destination = os.path.dirname(helper)
    libraries = collect(helper)
    if not libraries:
        raise SystemExit("没有收集到任何依赖，脚本逻辑需要复查")

    for name, source in libraries.items():
        target = os.path.join(destination, name)
        # 幂等：源与目标可能是同一个文件（重复执行本脚本时，helper 的依赖已被改成
        # @loader_path，解析出来就是同目录下的库），此时 shutil.copyfile 会抛 SameFileError。
        if os.path.realpath(source) != os.path.realpath(target):
            shutil.copyfile(source, target)
        os.chmod(target, 0o755)

    # 1) 拷贝出来的库：改自己的 id，并把自身依赖也指向 @loader_path。
    for name in libraries:
        target = os.path.join(destination, name)
        run("install_name_tool", "-id", f"@loader_path/{name}", target, check=False)
        for dep_name in libraries:
            for old in {f"@rpath/{dep_name}", libraries[dep_name], os.path.realpath(libraries[dep_name])}:
                run("install_name_tool", "-change", old, f"@loader_path/{dep_name}", target, check=False)

    # 2) helper：依赖全部改成 @loader_path，并删掉指向构建机的 rpath。
    for name, source in libraries.items():
        for old in {f"@rpath/{name}", source, os.path.realpath(source)}:
            run("install_name_tool", "-change", old, f"@loader_path/{name}", helper, check=False)
    for root in rpaths(helper):
        if root.startswith(("/Users/", "/private/", "/home/", "/opt/")):
            run("install_name_tool", "-delete_rpath", root, helper, check=False)

    # 3) ad-hoc 重签名。
    for path in [*(os.path.join(destination, name) for name in libraries), helper]:
        run("codesign", "--force", "--sign", "-", path, check=False)

    print("完成。helper 依赖现状：")
    for dep in dependencies(helper):
        print("  ", dep)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
