# console-web

Stage 3 / Slice 3-4 的最小 Console Web v0 已在此目录落地。

当前实现目标是一个 **只读管理面板**。它已经开始请求 live `Console API` 数据，对齐 `packages/runtime-core/console_api.go` 的返回结构，覆盖以下视图：

- 登录占位页头
- 运行状态
- 插件列表
- 任务列表
- 日志查看
- 配置查看

## 本地运行

```bash
npm install --workspace @bot-platform/console-web
npm run dev --workspace @bot-platform/console-web
```

## 构建

```bash
npm run build --workspace @bot-platform/console-web
```

## 设计边界

- 当前是 live API 驱动的只读面板，不是完整控制台
- 不包含真实登录
- 不包含写操作与控制面动作
- 不引入路由复杂度
- 重点是先稳定只读运行态展示，而不是提前扩展复杂控制面能力

## 当前状态说明

- 已不再是纯本地 mock 预览
- 已通过 `VITE_CONSOLE_API_URL` 或默认 `/api/console` 请求运行时 JSON 数据
- 已在前端消费层增加最小 payload shape 校验，避免对 live Console API 响应做盲目类型断言
- 已补最小前端契约测试，验证只读面板会请求 live Console API 并渲染关键只读内容
- 日志查看已支持 `log_query` 只读筛选，并与 URL 参数同步
- 任务列表已支持 `job_query` 只读筛选，并与 URL 参数同步
- 插件列表已支持 `plugin_id` 单插件定位输入，并与 URL 参数同步，用于直接查看该插件最近一次 failure / recovery 证据
- 插件表已最小显示当前进程内 `lastRecoveredAt`、`lastRecoveryFailureCount`、`currentFailureStreak`，用于补充恢复后最近一次恢复时间、恢复前失败次数与当前连续失败数
- 插件列表区块已补只读 `attention summary` 与默认关闭的 `attention-only` toggle，仅基于当前返回的 `plugins` 结果集统计 failing / recovered 并做本地过滤
- 空白 `plugin_id` 查询会退回未过滤请求，未知 `plugin_id` 仅显示空结果；`attention-only` 也只作用于当前结果集，不扩展成完整插件历史系统
- 仍属于 Stage 3 的最小版能力，不应按生产化控制台表述

## 下一步

- 在保持只读前提下补更真实的运行态展示
- 等读面稳定后，再评估是否引入更完整的控制台交互能力
