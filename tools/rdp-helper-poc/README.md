# RDP Helper（FreeRDP）

Qterm 的远程桌面（RDP）由**外部 helper 进程**提供：一个无窗口的 C 程序，链接 FreeRDP 3，
把远端桌面渲染到自己内存里的 GDI framebuffer，并通过 stdout 以"脏矩形帧"的形式交给 Qterm。

## 目录内容

- `freerdp/`：helper 源码（`main.c`）与 CMake 工程。构建产物在 `freerdp/build/`（已 gitignore）。
  当前使用的二进制就是 `freerdp/build/qterm-freerdp-helper`。

## 为什么只保留 FreeRDP

2026-10-14 对三条候选路线做过实测对照（详见 `docs/development/tasks/rdp-01.md` 的「引擎选型结论」）：

| 方案                 | 结论                                                                                         |
| -------------------- | -------------------------------------------------------------------------------------------- |
| **FreeRDP C helper** | **采用**。在必验目标（Windows Server 2008 R2 / RDP 7.1）上画面可读、键鼠生效                 |
| IronRDP Rust helper  | 该目标上能力协商后**解 PDU 失败**（`received 22 bytes, expected 120 bytes`），拿不到任何画面 |
| ironrdp-web（WASM）  | 必须自建 RDCleanPath WebSocket↔TCP 网关，且继承同一引擎行为，故不作首选                      |
| 自研纯 Go 栈         | 渲染保真度不足、会话内服务未打通，已废弃                                                     |
| gordp                | fork 回归导致任何服务器都连不上；缺 Orders/光标；明文口令写进 stdout 日志                    |

被移除的方案源码移到 `.cache/removed-approaches/`（`.cache` 被 gitignore，Go 工具链忽略点开头目录），
需要回溯时可从那里取回。

## 帧协议（helper → Qterm，stdout）

每帧 = **44 字节头 + 像素负载**，全部小端：

```
sequence(8) 桌面宽(4) 桌面高(4) x(4) y(4) 宽(4) 高(4) stride(4) 格式(4，2=RGBA8888) 负载长度(4)
```

负载是 `stride × 高` 字节的自顶向下 RGBA（`stride = 宽 × 4`）。

## 输入协议（Qterm → helper，stdin）

1. 启动后先按行写 FreeRDP 命令行参数，**空行结束**；密码只走这里，不进进程参数/环境变量/日志。
2. 之后是固定 **24 字节**二进制事件：`type(4) x(4) y(4) value(4) flags(4) pad(4)`，
   `flags`：bit0=按下、bit1=（滚轮）水平 /（键盘）扩展键。`type`：

| type | 含义                               | x / y / value / flags                                            |
| ---- | ---------------------------------- | ---------------------------------------------------------------- |
| 1    | 鼠标移动                           | x, y                                                             |
| 2    | 鼠标按键                           | x, y, value=按键（0=左 1=右 2=中 3=X1 4=X2）                     |
| 3    | 滚轮                               | x, y, value=格数（带符号）                                       |
| 4    | 键盘扫描码                         | value=扫描码                                                     |
| 5    | 关闭                               | —                                                                |
| 6    | Unicode 键盘                       | value 低 16 位 = UTF-16 码元（BMP 之外由上层拆成代理对分两次发） |
| 7    | 发布本机剪贴板文本                 | value = **紧随其后的 UTF-8 字节数**（事件之后真的有尾部负载）    |
| 8    | 请求重发远端剪贴板                 | 无负载、其余字段忽略                                             |
| 9    | 请求改分辨率（动态分辨率，不重连） | x=宽、y=高（合法范围 640×480 ~ 7680×4320）                       |

`type=7` 的"声明长度 + 尾部负载"必须成对且连续写出：上层（Go）在同一把写锁里先写
24 字节事件、再写负载，否则 helper 会把负载的头几个字节当成下一个事件，事件流永久错位。
超限时 helper 会**把声明的字节读干净再丢弃**（避免残包污染流），并打印一行警告。

## 剪贴板通道（CLIPRDR）

双向都只支持**文本**格式（Windows 侧 `CF_UNICODETEXT`），**不做文件剪贴板、图片剪贴板**；
单个方向上限 **1 MiB**，超限拒绝而不是截断（Qterm 侧 `helperClipboardMaxBytes` 与
helper 侧 `CLIPBOARD_TEXT_MAX_BYTES` 必须保持一致）。

- **本机 → 远端**：Qterm 发 `type=7`（文本 + 字节数）。helper 保存后向服务器发一次格式清单，
  用户在远端按粘贴时服务器回来索取这份文本。
- **远端 → 本机**：helper 把远端文本以**两条 stderr 契约行**上报，一行一条：

```
QTERM-CLIPBOARD-B64 <base64(UTF-8 文本)>
QTERM-CLIPBOARD-EMPTY
```

文本里可能有换行/控制字符，直接打印会破坏"一行一条"的协议，所以必须先 base64；
`-EMPTY` 表示"远端没有文本/被清空"。helper 平时按内容**去重**（内容没变不重复上报）；
Qterm 要主动刷新时发 `type=8`，helper 跳过这次去重、把当前远端剪贴板原样再上报一遍
（还没拿到过任何格式时回 `-EMPTY`）。

方向限制：**远端 → 本机依赖服务器把该会话的剪贴板格式清单推给 helper**。
服务器零推送的会话（典型例子：目标机上没有交互式登录会话）拿不到任何内容，
此时手动刷新也只会得到 `QTERM-CLIPBOARD-EMPTY`——这是目标的会话状态，不是 helper 的缺陷。

安全口径：剪贴板文本可能含敏感内容，Qterm 侧只把它留在内存
（`helperSession.remoteClipboard`）并走契约行上报，**不**写进 stderr 诊断文本、日志或错误消息。

## 构建

```bash
cmake -S tools/rdp-helper-poc/freerdp \
  -B tools/rdp-helper-poc/freerdp/build \
  -DFREERDP_SOURCE_DIR="$PWD/.cache/freerdp-src" \
  -DFREERDP_BUILD_DIR="$PWD/.cache/freerdp-build-make"
cmake --build tools/rdp-helper-poc/freerdp/build
```

构建后**必须**运行下面的脚本把动态库随包化：

```bash
python3 tools/rdp-helper-poc/scripts/relocate-dylibs.py
```

它会把 helper 依赖的 `libfreerdp*/libwinpr*`（当前 3 个）拷到 helper 同级目录，
把所有引用改成 `@loader_path/...`、删掉指向构建机的 `LC_RPATH`，并做 ad-hoc 重签名
（Apple Silicon 上 `install_name_tool` 会让原签名失效，不重签无法加载）。
执行完 `otool -L` 只应看到 `@loader_path/...` 与系统库。

打包时把 `freerdp/build/` 整个目录（helper + 那 3 个 dylib）随应用分发，
可用 `QTERM_RDP_HELPER_FREERDP=/abs/path` 覆盖 helper 位置。

## 待办（正式发布前必须处理）

1. ~~证书静默放行~~ **已实现**：helper 用 FreeRDP 的 `VerifyX509Certificate` 回调 + `FreeRDP_ExternalCertificateManagement=TRUE`
   做**指纹固定**，stderr 上报 `QTERM-CERT-SHA256/SUBJECT/ISSUER`，并以 `NEED-TRUST`/`MISMATCH` 拒绝连接（退出码 5）；
   Qterm 侧按 TOFU 询问用户并把指纹存本机。仍保留 `/tls:seclevel:0`（旧服务器需要）。
2. 动态库随包分发 + rpath 改写（见上）。
3. ~~剪贴板通道未接入~~ **已实现（文本、双向）**：输入侧 `type=7/8`，输出侧
   `QTERM-CLIPBOARD-B64`/`QTERM-CLIPBOARD-EMPTY` 两条契约行，上限 1 MiB、不做文件剪贴板（见上）。
   仍未接入的是**音频（rdpsnd）**——helper 协议目前还没有音频事件与音频契约行。
