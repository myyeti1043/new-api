# Implementation Plan: 敏感词过滤增强（Phase 2-4）

**Feature Directory**: `010-sensitive-filter-p2-4`
**Date**: 2026-06-02
**Spec**: [spec.md](./spec.md)
**Spec Repository**: `/mnt/g/project-github/new-api` (Git, docs/specs)
**Implementation Repository**: `/mnt/g/project-github/new-api` (Git)

## 生成前必读

已读取并执行 `.specify/memory/constitution.md`。确认：

- **语言约束**：实质内容使用中文，模板标题和代码标识符保留英文。
- **Bellman 状态杠杆**：已在 State Transition Plan 中记录完整字段。
- **风险门禁**：本次变更不涉及生产数据迁移或主路径读切换，属于功能增强，风险等级为中低。
- **阶段纪律**：plan 从已通过 checklist 的 spec.md 推导，未扩大范围。
- **任务格式**：tasks.md 将遵循可验证、文件/产物范围清晰的要求。

## Summary

**目标**：将现有简单敏感词过滤升级为支持输出侧过滤、分级规则和 PII 检测的企业级内容安全系统。

**当前状态**：Phase 1（审计日志）已完成。现有系统仅支持输入侧扁平敏感词列表检查，无输出侧过滤、无分级、无 PII 检测。

**目标状态**：支持输入/输出双向过滤、block/warn/log 三级规则、PII 检测与脱敏、CSV/TXT 批量导入导出。

## Planning Guard

| Gate | Status | Evidence / Decision |
|------|--------|---------------------|
| Active feature resolved from `.specify/feature.json` | PASS | `010-sensitive-filter-p2-4` |
| Git branch and active feature separated | PASS | `ACTIVE_FEATURE=010-sensitive-filter-p2-4`; `REPO_GIT_BRANCH=feat/sensitive-audit` |
| Constitution read before generation | PASS | `.specify/memory/constitution.md` 已读取 |
| Bellman state transition recorded | PASS | 见 State Transition Plan |
| Umbrella vs executable spec classified | PASS | 可执行规格（非 umbrella） |

## Technical Context

**Language/Version**: Go 1.21+, TypeScript/React (前端)
**Primary Dependencies**: gin (HTTP), GORM (ORM), Aho-Corasick (敏感词匹配), regexp (PII 检测)
**Storage**: MySQL/SQLite (Options 表存储配置, Log 表存储审计日志)
**Testing**: go test (后端), bun run build (前端编译验证)
**Target Platform**: Linux server + 现代浏览器
**Constraints**: 向后兼容现有扁平敏感词列表; 流式响应不阻断; go:embed 需重新编译前端+Go 二进制
**Scale/Scope**: Phase 2 (输出侧过滤) + Phase 4 (分级规则) + Phase 3 (PII 检测)

## Constitution Check

| 宪章约束 | 状态 | 证据 / 处理方式 |
|----------|------|-----------------|
| Bellman 状态杠杆决策 | PASS | 见 State Transition Plan |
| 中文文档默认 | PASS | 实质内容使用中文 |
| Spec 先于 plan/implementation | PASS | spec.md 已通过 requirements checklist |
| 高风险门禁与回滚 | N/A | 功能增强，不涉及生产数据迁移或主路径切换 |
| 可验证、文件/产物范围清晰的任务 | PASS | tasks.md 将按宪章要求格式化 |
| 阶段纪律与可追溯性 | PASS | 每个 task 追溯到 FR 和 plan item |

## State Transition Plan

| Field | Decision |
|-------|----------|
| Current State | Phase 1（审计日志）已完成。系统仅支持输入侧扁平敏感词列表检查，无输出侧过滤、无分级、无 PII 检测。`CheckSensitiveOnCompletionEnabled` 被注释掉。 |
| Target State | 支持输入/输出双向敏感词过滤（流式仅 log，非流式可 block）；支持 block/warn/log 三级规则体系；支持 PII 检测与脱敏（双向 mask）；支持 CSV/TXT 批量导入导出；现有扁平词列表作为 block 级别后备。 |
| Main Bottleneck | 需要修改多个 relay handler（OpenAI/Claude/Gemini/Responses）以累积响应文本，改动面较广但每个 handler 改动量小。 |
| Sequencing | Phase 2（输出侧）→ Phase 4（分级规则）→ Phase 3（PII 检测）。Phase 4 的数据结构影响 Phase 3 的 PII 规则集成。 |
| Scope | 后端：setting/sensitive.go, setting/pii.go(新), model/option.go, service/sensitive.go, service/pii.go(新), service/sensitive_import.go(新), controller/relay.go, controller/sensitive.go(新), relay/common/relay_info.go, relay/channel/openai/*, relay/channel/claude/*, relay/channel/gemini/*, router/api-router.go。前端：sensitive-words-section.tsx。 |
| Verification | go build 编译通过; bun run build 前端编译通过; 手动测试输入侧/输出侧过滤、分级规则、PII 检测; 审计日志正确写入 |
| Rollback Risk | 低。新增功能通过开关控制，默认关闭。回退只需禁用开关或回退代码版本。现有扁平词列表保持不变。 |

## Phase Plan

### Phase 0: Research / Decision Closure

无需额外研究。技术方案基于现有代码库模式：
- Aho-Corasick 引擎已验证可用
- Options 表配置加载模式已存在
- 审计日志写入机制已建立
- Relay handler 响应文本累积模式在各 handler 中已有实现

**已决策项**（来自 clarify 阶段）：
1. 规则唯一性：word + group 联合唯一
2. 流式输出侧：仅 log，不阻断
3. 非流式输出侧：支持 block + log
4. PII 脱敏：输入+输出双向
5. 旧词列表迁移：作为 block 级别后备

### Phase 1: Design Artifacts

- `research.md`: 不需要（技术方案基于现有代码库模式）
- `data-model.md`: 需要生成（定义 SensitiveRule、PIIConfig 等实体）
- `contracts/`: 需要生成（敏感词规则 CRUD API）
- `quickstart.md`: 需要生成（功能使用指南）

### Phase 2: Task Readiness

tasks.md 可以生成。所有 spec 歧义已通过 clarify 解决，技术方案基于现有代码库模式，无阻断项。

## Rollout And Rollback

**上线策略**：
1. 所有新功能默认关闭（CheckSensitiveOnCompletionEnabled=false, PIIEnabled=false）
2. 管理员手动开启各功能开关
3. 现有扁平敏感词列表保持不变，自动作为 block 级别后备

**回滚路径**：
1. 功能开关回滚：禁用 CheckSensitiveOnCompletionEnabled / PIIEnabled
2. 代码回滚：git revert 到 Phase 1 状态
3. 数据回滚：无需数据迁移，无破坏性变更

**证据要求**：
1. go build 编译通过
2. bun run build 前端编译通过
3. 现有敏感词检查功能不受影响（回归测试）
4. 新增审计日志正确写入

## Verification

- `go build -o new-api .` — 后端编译通过
- `cd web/default && bun run build` — 前端编译通过
- `go vet ./...` — 静态检查通过
- 手动测试：输入侧敏感词检查（现有功能回归）
- 手动测试：输出侧敏感词检查（新功能）
- 手动测试：分级规则 block/warn/log 行为
- 手动测试：PII 检测与脱敏
- 手动测试：CSV 导入导出
- 手动测试：审计日志查询页面显示正确

## Open Risks

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| Relay handler 改动面广（4 个 handler） | 中 | 每个 handler 改动量小，模式一致 |
| PII 正则误报 | 低 | 管理员可禁用特定 PII 类型 |
| 流式响应无法阻断 | 低 | 已在 spec 中明确为 log-only，用户预期已对齐 |
| go:embed 需重新编译 | 低 | 已知流程，前端+Go 二进制需同时重建 |
