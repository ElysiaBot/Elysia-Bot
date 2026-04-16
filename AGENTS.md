# AGENTS.md

## Mission
把当前仓库持续收敛为一个**长期运行、可恢复、可观测、可继续开发插件**的机器人运行平台。

默认优先级：
1. 主链可运行、可验证
2. 正确性与恢复语义
3. 局部收口速度
4. 边界清晰
5. 体验优化与扩展

出现冲突时，优先保证前四项，不为“看起来更完整”牺牲真实推进速度。

## Architecture Guardrails
长期边界：
- `packages/runtime-core` 承担共享运行语义、调度、状态、恢复、观测的核心边界
- `adapters/*` 负责协议接入与 transport/client 适配，不承载业务规则
- `plugins/*` 提供业务能力，不反向依赖 runtime 内部实现
- `apps/runtime` 负责集成与开发态入口
- `apps/console-web` / `Console API` 当前仍以读面为主，不包装成完整控制面

额外约束：
- 不把 adapter 直接写成业务 orchestrator
- 不绕过 runtime 直接让 adapter 操作 plugin 语义
- 涉及事件、任务、工作流、外部调用时，优先考虑 idempotency / retry / timeout / replay / audit
- 插件运行边界与 subprocess host 变化，优先保持可诊断与可恢复