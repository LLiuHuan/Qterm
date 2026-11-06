# 后端导航

进入 `internal` 后先按**功能**找包：主机、会话、终端、SFTP、AI 各有一个一级目录；
`app` 是唯一 Wails facade，`model` / `store` / `vault` / `tunnel` 等提供共享模型与基础设施。
分层、错误码、并发与资源回收的完整规则见内部仓库的 `docs/WWORKBENCH-CODING-STANDARD.md`
（公开快照不含该文档）。

## 目录

```text
main.go                     只嵌入前端资源并调用 app.Run
internal/
  app/                      唯一 Wails facade：装配、窗口、事件桥、各功能的边界适配
  host/                     主机、分组、快捷命令的用例与校验
  credential/               凭据记录、秘密消费边界与导出快照
  security/                 本地数据加密：DEK、系统钥匙串、恢复包
  vault/                    加密存储与密钥包装
  session/                  会话所有权、连接编排、主机密钥确认、认证
  ssh/                      SSH 客户端与认证细节
  terminal/                 PTY、终端帧协议、一次性令牌与背压
  sftp/                     SFTP 浏览、上传下载队列与覆盖确认
  tunnel/                   SSH / PTY / SFTP 等传输适配（可替换基础设施）
  proxy/                    代理库（SOCKS5 / HTTP）
  remotedesktop/            RDP 会话集成、helper 帧协议与证书校验
  vnc/                      VNC 会话
  x11/                      本机 X Server 检测与 X11 转发
  zmodem/                   ZMODEM 拦截与事件
  docker/                   远端 Docker 命令面、解析与详情
  ai/                       AI 供应商配置、对话请求、Agent 运行与命令策略
  otp/                      TOTP / HOTP 条目与取码
  translate/                右键翻译
  update/                   GitHub Release 自更新
  hostimport/               第三方客户端配置导入（OpenSSH / PuTTY / Xshell 等）
  assetdata/                资产快照
  assettransfer/            资产导入导出
  resourcebundle/           逻辑资产迁移格式
  recording/                终端录制文件读写
  recordingsdir/            录制保存目录解析
  remoteedit/               本机编辑器编辑远端文件
  preview/                  SFTP 远端图片预览
  fonts/                    本机字体枚举
  diagnostics/              本机调试日志（默认关闭）
  preferences/              界面偏好（不含秘密）
  applock/                  应用锁（独立于保险库主密码）
  model/                    跨功能领域模型与纯规则
  store/                    SQLite 持久化与迁移
  sqlitedsn/                SQLite DSN 构造
  errno/                    业务错误协议
```

## 按功能查代码

### 连接与认证

- `session`：会话状态、连接所有权、重连与主机密钥确认；连接器在同目录，
  避免把 SSH 实现拆成跨目录的隐式依赖。
- `ssh`：SSH 客户端与认证方式（密码 / 私钥 / Agent / 交互式 / OTP 挑战）。
- `terminal`：二进制帧协议、PTY Stream、ACK 背压与关闭处理；通过会话所有者接口工作，
  不反向依赖 `session` facade。
- `sftp`：目录浏览、文件选择与传输队列；协议实现位于 `tunnel`。

### 数据与安全

- `security` 负责 DEK 与系统钥匙串，`vault` 负责加密记录与主密码包装；两者都不出现在前端 DTO 里。
- `store` 只做 SQL、迁移与 scan/encode；业务规则留在功能包。
- 秘密不得进入日志、错误详情、事件载荷或测试快照；只写输入 DTO 在功能包内消费完即丢弃。

### 共享与边界

- `app`：构造服务、注入依赖、连接跨功能回调，并提供唯一 Wails facade。方法保持薄转发，
  不写 SQL / SSH / 文件传输 / 复杂业务规则。
- `model`：跨功能模型与纯规则，不导入 Wails 或基础设施。
- `errno`：业务错误协议（`code` / `messageKey` / `safeDetail`），用户文案由前端语言包负责。

依赖方向保持单向：`main → app → 功能包 → model / store / vault / tunnel`；
基础设施不反向调用功能包。

## 新代码放哪里

- 只服务某个功能的私有逻辑，放对应功能目录，按用途拆文件（`service.go`、`manager.go`、`*_feature.go`）。
- 业务校验与状态转换放 `model`；用例编排与长生命周期资源放功能包的 Service / Manager。
- 文件、网络、进程、密钥包装和数据库操作放 `store`、`vault`、`tunnel`。
- 只有多个功能确实复用、且不含业务状态的纯函数，才新建按能力命名的小包。
  不要预建 `utils`、`common` 或 `helpers` 大杂烩。

新增方法时先确认它服务哪个功能，再决定是否导出；导出的类型与函数要有清晰注释。

## 本地校验

```sh
gofmt -l ./internal        # 应无输出
go build ./internal/...
go vet ./internal/...
go test ./internal/...
```

跨边界改动（facade / DTO）还要重新生成绑定：`wails3 generate bindings -clean=false -ts -i`。
