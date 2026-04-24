# runtime-storage-main-path

这页把当前仓库里与 runtime storage 主路径、Postgres 非默认验证路径、以及相邻的 AI provider 路径分类相关的成熟度口径收口到一个地方。

当前真值以这些 repo-owned 文件为准：`README.md`、`docs/roadmap/README.md`、`deploy/config.dev.yaml`、`package.json`、`.github/workflows/nightly-validation.yml`。

## 1. 当前主路径事实

- `package.json` 把 `npm run dev:runtime` 定义为 `go run ./apps/runtime -config ./deploy/config.dev.yaml`，所以当前默认开发态 runtime 直接读取这份配置。
- `deploy/config.dev.yaml` 默认写的是 `runtime.sqlite_path: data/dev/runtime.sqlite`。
- 同一份配置把 `runtime.smoke_store_backend` 设为 `sqlite`，注释也明确说切到 `postgres` 只是给 v0 smoke path event / idempotency verification 用。
- `README.md` 把 SQLite 写成当前主要状态存储，用来承载 event journal、idempotency、plugin 状态、job、schedule、alert、audit 等运行态数据。

因此，SQLite 是当前 runtime storage 的 main path。它不是若干默认 store 之一，而是当前默认启动、默认配置、默认叙事共同指向的主存储。

## 2. Postgres 的当前分类

- `deploy/config.dev.yaml` 默认 `runtime.postgres_dsn: ""`，不会在默认开发态入口里主动切到 Postgres。
- `README.md` 和 `docs/roadmap/README.md` 都把 Postgres 描述成 smoke / acceptance 或 non default 路径，而不是当前主存储。
- `package.json` 提供 `npm run test:postgres:smoke` 与 `npm run test:postgres:acceptance`，说明仓库确实在维护一条真实的 Postgres 验证线。
- `.github/workflows/nightly-validation.yml` 里的 `postgres-smoke` job 会拉起 Postgres service，并每晚执行这两条命令。

因此，Postgres 的当前分类应该是 verified but non-default path。它用于 smoke 和 acceptance 验证，不是当前默认 store，也不是已经收口成多存储产品面。

## 3. 当前验证如何支撑这个分类

- `npm run test:postgres:smoke` 当前运行 `go test ./packages/runtime-core -run 'TestPostgresStoreLiveRoundTrip' -count=1`，证明 runtime core 层确实有 live Postgres store round trip 验证。
- `npm run test:postgres:acceptance` 要求 `BOT_PLATFORM_POSTGRES_TEST_DSN`，并在 `./apps/runtime` 跑一组聚焦 acceptance 测试，覆盖 demo message、direct host、replay、workflow restore、operator write path 等当前收口中的 runtime 语义。
- nightly workflow 不是只跑 smoke。它在同一个 `postgres-smoke` job 里先跑 `test:postgres:smoke`，再跑 `test:postgres:acceptance`，所以 Postgres 分类既有本地命令面，也有 nightly evidence。

这说明 repo 对 Postgres 的态度是“有验证、能回归、但不当默认主线”。文档上应该保持这个口径。

## 4. 相邻的 AI provider 路径分类

- `deploy/config.dev.yaml` 默认还是 `ai_chat.provider: mock`，所以当前本地直接启动的默认 AI 路径是 `mock`。
- 同一份配置保留了切到 `openai_compat` 的最小真实 provider 入口，同时要求 `secrets.ai_chat_api_key_ref`，并要求非 `localhost` / loopback 场景使用 `https://` endpoint。
- `README.md` 与 `docs/roadmap/README.md` 都把 `openai_compat` 描述成当前窄 real provider path，不把它写成更宽的 provider platform 或多供应商框架。

因此，这条相邻分类应该这样理解：`mock` 是默认启动路径，`openai_compat` 是已存在、已收口、但范围很窄的真实 provider 路径。它证明 runtime 已经接住一条 real provider integration，不代表仓库已经承诺更大的 provider abstraction。

## 5. 当前非目标

- 这份 topic doc 不把当前仓库写成多 store 产品。
- 它不把 Postgres smoke / acceptance path 叙述成“默认存储随时可切换”的广义 storage abstraction。
- 它不把 `openai_compat` 叙述成通用 provider framework、多供应商平台或完整密钥管理产品面。
- 它不把多节点、分布式执行、跨节点 storage 扩张写成当前主路径。`docs/roadmap/README.md` 当前也明确把这些内容放在非当前承诺边界之外。
- 它不为了未来生态提前做更大的 storage / provider 抽象。
