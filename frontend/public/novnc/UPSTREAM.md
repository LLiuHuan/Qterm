# noVNC：来源与许可（**现已改为 npm 依赖，见下**）

> **2026-10-16 变更**：本目录的 vendored 拷贝**已不再被代码引用**。原因是 `public/` 里的文件
> 不能被 Vite 当模块导入（dev 下会报 "This file is in /public ... should not be imported
> from source code"），导致 dev 直接连不上 VNC。现在改为 npm 依赖
> **`@novnc/novnc@1.7.0`**（`package.json` 锁定精确版本，`exports: ./core/rfb.js`），
> 由 Vite 正常打包，dev 与生产行为一致。本目录保留作为来源记录，
> **可在确认后删除**（删除属于批量操作，按仓库约定先征求同意）。

## 原 vendored 说明

本目录是**原样复制**的上游 noVNC 库文件，供 Qterm 的 VNC 会话（ADR-017 路线 C）在
WebView 内直接以 ES module 加载使用：`import RFB from '/novnc/core/rfb.js'`。

**请勿修改本目录里的任何文件。** 需要定制行为时，用我们自己的包装层（例如传入
`RFB` 的选项、或在 pane 组件里设置 `rfb.viewOnly`），不要改上游文件——MPL-2.0 是
**文件级** copyleft：修改过的 MPL 文件必须公开其改动源码，不改就只需保持许可与来源。

## 来源与版本

| 项       | 值                                                                                                                               |
| -------- | -------------------------------------------------------------------------------------------------------------------------------- |
| 上游仓库 | https://github.com/novnc/noVNC                                                                                                   |
| 提交     | `4a2cc175ae8af50fe8cb85a05d40908c01c43f6a`（2026-10-16 拉取）                                                                    |
| 复制范围 | `core/**`（RFB 客户端库）、`vendor/pako/**`（zlib，`vendor/pako` 为 MIT）                                                        |
| 未复制   | `app/**`（HTML/CSS 为 BSD-2、`app/images/**` 为 CC-BY-SA 3.0、Orbitron 字体为 OFL）——我们压制上游自带 UI，因此这些素材不随包分发 |

## 许可证

- noVNC 核心库（`core/**`）：**MPL-2.0**，全文见同目录 `LICENSE.txt`
- `vendor/pako/**`：MIT（见 `LICENSE.txt` 中列出的许可文本）
- 发行包必须包含 `LICENSE.txt`，并在应用的第三方许可/关于页登记 noVNC 与版本号

升级步骤：`git clone --depth 1 https://github.com/novnc/noVNC` → 记录新提交 →
整体替换 `core/` 与 `vendor/pako/` → 更新本文件与上表的提交号 → 跑一次真实 VNC 会话冒烟。
