# Qterm Homebrew Tap 分发

发布工作流从最终签名后的两个 macOS ZIP 实算 SHA256，并与发布清单校对，生成双架构 `qterm.rb`。配方会附在 GitHub Release，同时保留为 Actions 的 `qterm-homebrew-cask` artifact。配方下载地址自动采用实际运行工作流的仓库，不需要把账号写死在源码里。

这是一条自建 Tap 分发路径，不代表已收录到 Homebrew 官方仓库。ad-hoc 签名不包含 Apple 公证，通过 Homebrew 安装仍可能需要在 macOS「系统设置 → 隐私与安全性」中允许打开；配方不会清除隔离属性或关闭 Gatekeeper，也不会删除应用数据。

## 一次性配置自动更新

1. 在 GitHub 建立公开仓库 `<账号>/homebrew-tap`，勾选添加 README，确保默认分支已有初始提交。已经有 Tap 的可以复用。
2. 建立 fine-grained personal access token，只选择这个 Tap 仓库，授权 **Contents: Read and write**（Metadata 读取随附）。在组织仓库中可能需要管理员批准。该凭据不要写入文件、命令或日志。
3. 在 **Qterm 发布仓库**的 Settings → Secrets and variables → Actions 中设置：

   | 类型                      | 名称                      | 内容                                |
   | ------------------------- | ------------------------- | ----------------------------------- |
   | Repository variable       | `HOMEBREW_TAP_REPOSITORY` | `<账号>/homebrew-tap`               |
   | Repository secret         | `HOMEBREW_TAP_TOKEN`      | 上一步创建的 Token                  |
   | Repository variable，可选 | `HOMEBREW_TAP_BRANCH`     | 要更新的分支；不填使用 Tap 默认分支 |

4. 将本次发布源码推送至实际发布仓库，再发布包含这些改动的新 `vX.Y.Z` 标签。工作流先发布安装包，之后 `update-homebrew-tap` 自动创建或更新 `Casks/qterm.rb`。

默认 `GITHUB_TOKEN` 不能代替 Tap 跨仓库写入凭据。未设置 `HOMEBREW_TAP_REPOSITORY` 时自动更新 job 跳过，但配方仍照常生成。已配置目标仓库却缺少 Token、目标分支不存在或被保护规则禁止直接写入时，该 job 会明确失败，已经发布的安装包不会撤回。修复配置后可在 Actions 重跑失败的 job。

更新会检查旧版本，跳过降级和内容完全相同的重复提交。版本相同但最终包校验值改变时，允许更新配方；文件 SHA 用于防止覆盖并发改动，冲突时失败而不会强制覆盖。建议正式发布后不替换同版本安装包。多个发布仓库共享一个 Tap 时，也依靠文件 SHA 检测竞争，失败后需要重跑。

## 用户安装与更新

以下账号替换成 Tap 所属账号；仓库名 `homebrew-tap` 对应 Tap 名 `tap`：

```bash
brew tap <账号>/tap
brew install --cask <账号>/tap/qterm
```

新版本发布并同步 Tap 后：

```bash
brew update
brew upgrade --cask <账号>/tap/qterm
```

Homebrew 会根据 CPU 选择 arm64 或 amd64 ZIP，并校验对应 SHA256。首次通过 Tap 安装前，确认配方内容、下载地址和仓库来源。已手动安装 `/Applications/qterm.app` 时，Homebrew 可能提示应用已存在，需要先自行决定保留哪一份安装；不要直接使用强制覆盖命令。

## 不配置 Token 的手动维护

从 Release 下载 `qterm.rb`，放到自己的 Tap 仓库 `Casks/qterm.rb`，检查后自行提交和推送即可。每个版本重复此操作，无需给 Qterm 发布流程跨仓库权限。

也可以用本地最终产物生成（目录须含两个签名后的 ZIP 和 `SHA256SUMS`）：

```bash
node build/homebrew/generate-cask.mjs \
  --repository <账号>/<Qterm发布仓库> \
  --tag v0.0.5 \
  --artifacts bin/artifacts \
  --output /tmp/qterm-tap/Casks/qterm.rb
```

`v0.0.5` 仅为示例，必须与实际 Release 和产物版本一致。配方使用 ZIP 的摘要，不使用 DMG 摘要，也不使用签名前中间包的摘要。生成器验证完整性，真实签名验证由上游 macOS job 完成。

## 验证

```bash
node --test build/homebrew/generate-cask.test.mjs build/homebrew/update-tap.test.mjs
ruby -c /tmp/qterm-tap/Casks/qterm.rb
```

在真实 Tap 发布后，还需要分别验证 Apple Silicon / Intel 安装、升级和首次打开。只通过 Node 用例和 Ruby 语法检查，不能声称这些线上安装步骤已通过。

参考：[Homebrew Cask Cookbook](https://docs.brew.sh/Cask-Cookbook)、[GitHub Actions Token 权限](https://docs.github.com/en/actions/tutorials/authenticate-with-github_token)。
