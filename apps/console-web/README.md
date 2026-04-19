# console-web

`apps/console-web` 现在是一个 **本地 operator console**，而不再只是单页只读预览。

它仍然坚持当前仓库边界：

- 读模型来自现有 `GET /api/console`
- 写操作走现有 runtime `/demo/*` operator endpoints
- 浏览器内的 operator identity 只是 **本地/dev actor ID**，通过现有 `X-Bot-Platform-Actor` header 传递
- 不引入新的 backend auth / session / login 系统

## 当前能力

- 轻量级路由化控制台：
  - `/`
  - `/plugins/:pluginId`
  - `/jobs/:jobId`
  - `/schedules/:scheduleId`
  - `/adapters/:adapterId`
  - `/workflows/:workflowId`
- 本地 operator identity 面板：
  - browser-local actor ID
  - 当前 snapshot 中的 roles / permissions 可视化
  - actor header 名称来自 runtime meta，而不是前端硬编码假装 auth
- 刷新模型：
  - 手动 refresh
  - last fetched / runtime generated 时间显示
  - 可关闭的轻量自动 refresh
- 当前仓库已支持的 operator 写操作：
  - plugin enable / disable
  - plugin-echo config update
  - dead-letter job retry
  - schedule cancel
- 更完整的读面证据：
  - alerts
  - audits
  - recovery
  - workflows
  - replay / rollout declarations
  - operator capability / limitation meta

## 本地运行

先启动 runtime：

```bash
npm run dev:runtime
```

再启动 Console Web：

```bash
npm run dev --workspace @bot-platform/console-web
```

Vite dev server 已做最小代理，默认把 `/api`、`/demo`、`/metrics`、`/healthz` 转到本地 runtime `http://127.0.0.1:8080`，方便浏览器手动 QA。

## 验证

```bash
npm run test --workspace @bot-platform/console-web
npm run build --workspace @bot-platform/console-web
```

## 设计边界

- 这是 **本地 operator console**，不是完整控制台产品
- browser-local actor identity 只是对当前 runtime actor-header 现实的诚实 UI 包装
- plugin config editor 故意只收窄到 `plugin-echo` 当前已存在的 persisted config 合同
- 所有写操作都走 read-after-write refetch，不做 optimistic authority 假象
- 不扩展成新的 control-plane API，也不重构 runtime 为前端服务
