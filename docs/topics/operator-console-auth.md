# operator-console-auth

这页把当前仓库里与 operator console、bearer auth、有限控制面写路径相关的成熟度口径收口到一个地方。

当前真值以这些 repo-owned 文档与配置为准：`README.md`、`docs/roadmap/README.md`、`apps/console-web/README.md`、`deploy/config.dev.yaml`。

## 1. 当前主路径事实

- 仓库当前确实包含 `apps/console-web`，它是 live API 驱动、读面优先的本地 operator console。
- Console Web 的读模型来自 runtime `GET /api/console`。本地开发态通过 Vite proxy 把 `/api`、`/demo`、`/metrics`、`/healthz` 转到同一个 runtime。
- 当前主路径里的 console 重点仍是运行态可见性。现有读面已经覆盖 runtime、adapter、plugin、job、schedule、log、config 等事实。
- operator identity 的权限真值不在前端，也不在 token claims。当前 actor ID、roles、permissions、plugin scope 仍由 runtime 的 RBAC 配置与持久化 snapshot state 决定。
- `deploy/config.dev.yaml` 已把 operator auth 写进默认开发配置，所以当前主路径可以明确承认，仓库已经有一条 repo-owned 的本地 operator auth 入口，而不是只靠临时 header 约定。

## 2. 已验证，但不是更宽默认承诺的范围

- runtime 当前已经提供一批最小 operator 写入口，用来证明控制面写路径成立，而不是把控制台包装成完整产品：plugin enable / disable、`plugin-echo` config update、delay schedule create / schedule cancel、queued job pause / resume / cancel、dead-letter job retry。
- Console Web 当前直接消费现有 runtime `/demo/*` operator endpoints，而不是定义一套新的 control-plane API。浏览器 UI 已接入其中一部分最小写操作，但这仍然是有限写面。
- 当前 bearer 基线来自 `deploy/config.dev.yaml` 里的 `operator_auth.tokens`。每个 token 通过 `token_ref` 绑定环境变量 secret，通过 `actor_id` 绑定已配置 actor。
- `Authorization: Bearer <token>` 当前只负责把请求绑定到一个已配置 actor identity。roles、permissions、plugin scope 仍以 `rbac.actor_roles`、`rbac.policies` 与 runtime persisted snapshot state 为真值。
- 当 `operator_auth.tokens` 已配置时，console read、Console Web 与当前 operator 写路径都按 bearer transport 理解。只有在 operator auth 未配置的兼容场景下，runtime 才回退到 `X-Bot-Platform-Actor` header。
- 这条 bearer 路径虽然已经进入当前 repo 默认开发配置，但它的成熟度仍应归类为“本地、有限、已验证”的 operator baseline，不应被写成更宽的认证产品承诺。

## 3. 非主线 / not now 边界

- 这里还没有 JWT 解析，也没有把 token claims 当成权限真值。
- 当前没有 SSO 集成。
- 当前没有 login UI、真实登录流程或完整 session 管理产品面。
- Console Web 已有轻量路由页与一部分写操作，但这不等于完整 control-plane 产品，更不代表 repo 已承诺批量操作、实时推送体系或更宽的运维平台能力。
- `docs/TODO.md` 里仍把“完整控制台产品化”放在非当前承诺范围内，所以这条 topic doc 只描述当前真实边界，不把远期平台面提前写成现在的主线。
