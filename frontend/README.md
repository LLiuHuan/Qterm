# Qterm 前端

Qterm 的界面层：Vue 3 `<script setup>` + TypeScript + Vite，样式以 Tailwind 原子类为主。
它由 Wails 桌面壳承载，通过 `@wailsio/runtime` 调用 Go 侧 facade 并接收事件。

## 与桌面壳的关系

- 生产构建产物（`dist/`）会被 `main.go` 用 `//go:embed` 打进二进制；开发时由 Wails 反向代理到 Vite。
- 所有后端能力都经 `src/services/*` 的薄封装调用生成的绑定（`bindings/`），组件里不直接 import 绑定。
- **运行时是延迟注入的**：开发模式下 `window._wails.environment` 可能晚于应用挂载才出现。
  需要"真后端 / 内存兜底"二选一的网关一律用 `services/runtime.ts` 的 `createRuntimeGateway`，
  不要在创建时一次性判定后缓存结果（详见规范 §21.8 与 `runtime.spec.ts`）。
- **纯浏览器预览只用于界面开发**：内存网关返回空数据，不能替代 SSH / SFTP / 远程桌面 / 保险库。

## 常用命令

在 `frontend/` 目录执行（依赖用 `npm ci` 安装）：

```sh
npm run dev          # 只跑 Vite（默认端口 9245），配合 Wails dev 或浏览器预览
npm run typecheck    # vue-tsc --noEmit
npm run lint         # eslint --max-warnings=0
npm run format:check # prettier 检查（format 为自动修复）
npm run test:unit    # vitest
npm run test:e2e     # playwright（需要先 npx playwright install chromium）
npm run build        # typecheck + 生产构建
npm run check:all    # 静态检查 + 单测 + 构建 + E2E
```

仓库根目录的 `wails3 task check` 等价于 `check:all`，是提交前的完整门禁。

## 目录结构

```text
src/app/          应用外壳、窗口入口与 shell 级状态
src/features/     按功能划分：hosts、sessions、sftp、ai、otp、settings、updates 等
src/components/   公共控件（ui/ 为 shadcn-vue 风格基件）、布局、对话框、工作区组件
src/services/     后端网关、运行时封装与浏览器兜底实现
src/locales/      七种语言的文案表（按领域分文件夹，每语言一个文件）
src/styles/       全局样式与主题覆盖（入口 index.css，导入顺序即级联顺序）
e2e/              Playwright 用例
bindings/         生成的 Wails TypeScript 绑定，禁止手改
```

## 编码约定（改动前先读）

- 样式：能用 Tailwind 原子类表达的写在模板 `class` 上；颜色走 `--qt-*` 令牌；只有复合选择器、
  伪元素、媒体查询、动画或第三方深层结构才写 CSS。详见规范 §22。
- 字号：只有 12 / 14 / 16 三档，层级差异用字重与颜色表达，不新增中间值。详见 §23。
- 图标：优先 `@lucide/vue`；没有合适图标时才从 iconify 落成本地 SFC 并跑 `node gen-icons.mjs`。详见 §24。
- 文案：新增键必须七种语言同时补齐（`src/locales/locale-coverage.spec.ts` 会校验键集、占位符、
  非中文语言不残留中文、不照抄英文）；后端错误通过 `messageKey` 交给前端翻译。
- 绑定：改 Go facade 后执行 `wails3 generate bindings -clean=false -ts -i`（等价于
  `wails3 task common:generate:bindings`），不要手改 `bindings/`。
- 接口与分层、错误码、并发与资源回收等后端规则见内部仓库的
  `docs/WWORKBENCH-CODING-STANDARD.md`（公开快照不含该文档）。

## 验证要求

涉及几何（对齐、等高、间距）或颜色的改动，不能只跑单测：要在真实浏览器里断言 computed style
与元素位置。仓库里的 `e2e/` 与临时 Playwright 脚本都按这个口径用。
