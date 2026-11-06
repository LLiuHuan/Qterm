# Qterm OTP 本机测试夹具

这个夹具启动一个只监听本机的 OpenSSH 服务，并用 PAM 强制执行密码 + TOTP。它用于验证 Qterm 的 SSH 交互认证，不连接外部服务器，也不挂载用户目录。

测试地址和账号固定为：

- 地址：`127.0.0.1:22232`
- 用户：`qtermotp`
- 密码：`qterm-otp-password`
- TOTP 密钥：`JBSWY3DPEHPK3PXP`
- 算法/位数/周期：`SHA1 / 6 / 30s`

这些值只适用于本地测试。不要把它们用于真实账号。

## 启动

在仓库根目录执行：

```sh
docker compose -f tools/otp-test/compose.yaml up -d --build
docker compose -f tools/otp-test/compose.yaml ps
```

容器名是 `qterm-otp-test`，端口只绑定到 `127.0.0.1:22232`。停止并移除这个测试容器时执行：

```sh
docker compose -f tools/otp-test/compose.yaml down
```

## 验证服务

使用夹具自带的检查脚本：

```sh
tools/otp-test/check.sh
```

脚本会先用临时 `known_hosts` 记录容器的主机密钥，再依次验证错误密码被拒绝、正确密码加错误 OTP 被拒绝、正确密码加当前 OTP 可以登录。错误场景只有在 SSH 明确返回 `Permission denied` 且确实见到认证提示时才算通过；超时、连接失败或没有挑战不会被当作“拒绝成功”。脚本不会关闭主机密钥检查，也不会把密码或验证码打印到输出。

## 在 Qterm 中测试

1. 启动容器后，在 Qterm 的凭据/OTP 管理里选择“识别二维码”，选择容器内的二维码：

   ```sh
   docker cp qterm-otp-test:/opt/otp-test/otpauth.png /tmp/qterm-otp-test.png
   ```

   然后选择本机的 `/tmp/qterm-otp-test.png`。也可以在新建表单内粘贴二维码图片；目前没有 URI 文本导入入口。

2. 保存识别出的 OTP 条目。OTP 是全局凭据，连接认证弹窗会从全部 OTP 中选择，不需要绑定主机。
3. 新建一个 SSH 主机：地址填 `127.0.0.1`，端口填 `22232`，用户名填 `qtermotp`，密码认证选择“连接时询问”。
4. 发起连接。认证弹窗应先显示密码提示，再显示验证码问题；输入 `qterm-otp-password` 和 Qterm 当前显示的 6 位验证码后应连接成功。
5. 故意输入与当前验证码不同的 6 位数字或过期验证码，连接应失败并保留可重试的认证错误。

Qterm 连接测试应开启主机密钥校验。首次连接会显示指纹确认，确认后才进入密码/验证码流程。

## 夹具文件

- `Dockerfile`：Debian + OpenSSH + `libpam-google-authenticator`。
- `google_authenticator`：公开测试密钥和 TOTP 参数。
- `sshd_config`、`pam-sshd`：只允许 keyboard-interactive PAM 认证。
- `totp.py`：容器内生成当前测试验证码，避免依赖额外客户端。
- `ssh-auth.exp`、`check.sh`：验证错误密码、错误 OTP 和正确组合的脚本。

夹具不验证 Windows/Linux 原生界面、不上传任何图片，也不测试真实供应商或真实 SSH 凭据。

也可以用临时连接 `qtermotp@127.0.0.1:22232` 测试，不必保存主机。首次凭据窗口和 PAM 密码问题都使用测试密码；验证码问题再选择全局 OTP 条目并填码，最后手动提交。夹具没有启用 SFTP 子系统。

## NyaTerm 对照参数与重复密码说明（2026-11-05）

- SSH 地址 `127.0.0.1`、端口 `22232`、用户 `qtermotp`、密码 `qterm-otp-password`。
- OTP 发行商 `Qterm OTP`、账号 `qtermotp`、类型 TOTP、Base32 密钥 `JBSWY3DPEHPK3PXP`、算法 SHA1、6 位、周期 30 秒；HOTP 计数器不适用，不需要绑定主机。
- 完整 URI：`otpauth://totp/Qterm%20OTP:qtermotp?secret=JBSWY3DPEHPK3PXP&issuer=Qterm%20OTP&algorithm=SHA1&digits=6&period=30`。
- 二维码本机路径 `/tmp/qterm-otp-test.png`。
- 当前 ED25519 指纹 `SHA256:WEz+nwfGvBxSzw8GCrgUdegJTAFgPQ8InrczGXRkeLc`；重建容器后需重新核对。
- 服务端关闭 PasswordAuthentication，只接受 keyboard-interactive:pam，依次要求 `Password:` 和 `Verification code:`。Qterm 当前会先收集普通 SSH 密码，这份密码没有复用到交互挑战，所以是“Qterm 首次密码 → PAM 密码 → PAM 验证码”；前两次填同一个测试密码。NyaTerm 若支持直接交互认证，可选择该方式，对比是否跳过首次通用密码弹窗。
- 成功后执行 `whoami` 应输出 `qtermotp`。本容器未配置 SFTP，SFTP 失败不代表 OTP 登录失败。
