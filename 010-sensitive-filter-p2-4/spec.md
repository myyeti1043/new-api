# Feature Specification: 敏感词过滤增强（Phase 2-4）

**Feature Directory**: `010-sensitive-filter-p2-4`
**Created**: 2026-06-02
**Status**: Draft
**Input**: 实现 Phase 2-4 的敏感词过滤功能改进：Phase 2 输出侧内容过滤、Phase 4 分级敏感词体系、Phase 3 PII 检测引擎

## User Scenarios & Testing *(mandatory)*

### User Story 1 - 输出侧内容过滤 (Priority: P1)

作为企业管理员，我需要系统在 LLM 返回的响应中也检测敏感词，以便满足企业合规要求，防止敏感信息通过 AI 响应泄露。

**Why this priority**: 输出侧过滤是企业合规的基础要求。目前系统只检查输入侧（用户 prompt），但 LLM 可能在响应中生成敏感内容。没有输出侧过滤，企业无法满足数据安全审计要求。

**Independent Test**: 启用输出侧过滤后，发送一个会触发 LLM 返回敏感词的请求，验证系统能检测到并记录审计日志。

**Acceptance Scenarios**:

1. **Given** 管理员已启用输出侧过滤，**When** LLM 响应包含敏感词，**Then** 系统记录审计日志（direction=output）
2. **Given** 管理员已启用输出侧过滤，**When** LLM 响应不包含敏感词，**Then** 响应正常返回，无额外日志
3. **Given** 输出侧过滤未启用，**When** LLM 响应包含敏感词，**Then** 响应正常返回，不记录输出侧日志
4. **Given** 流式响应模式，**When** LLM 逐步返回内容且包含敏感词，**Then** 系统在响应完成后检查完整内容并记录审计日志（不阻断，因 chunk 已发送）
5. **Given** 非流式响应模式，**When** LLM 响应包含敏感词，**Then** 系统在返回给用户前检查完整内容，根据配置执行阻断或记录

---

### User Story 2 - 分级敏感词管理 (Priority: P2)

作为企业管理员，我需要为不同敏感词设置不同的严重级别（阻断/告警/记录），以便对不同风险等级的内容采用不同处理策略，避免误杀低风险内容。

**Why this priority**: 分级体系是精细化管理的基础。当前所有敏感词一视同仁（要么阻断要么不管），无法区分政治敏感（需阻断）和一般不当内容（仅需记录）。

**Independent Test**: 添加一条 level=warn 的敏感词规则，发送包含该词的请求，验证请求通过但审计日志被记录。

**Acceptance Scenarios**:

1. **Given** 管理员添加了一条 level=block 的规则，**When** 用户请求包含该词，**Then** 请求被拒绝并记录审计日志
2. **Given** 管理员添加了一条 level=warn 的规则，**When** 用户请求包含该词，**Then** 请求通过但记录审计日志
3. **Given** 管理员添加了一条 level=log 的规则，**When** 用户请求包含该词，**Then** 请求通过并记录审计日志（低优先级）
4. **Given** 管理员通过 CSV 批量导入敏感词规则，**When** 上传包含 word,level,category 的 CSV 文件，**Then** 规则被成功导入
5. **Given** 管理员需要导出规则，**When** 点击导出按钮，**Then** 系统生成 CSV 文件下载

---

### User Story 3 - PII 检测与脱敏 (Priority: P3)

作为企业管理员，我需要系统自动检测请求和响应中的个人身份信息（手机号、身份证号、银行卡号、邮箱），以便防止 PII 数据泄露到外部 LLM 服务。

**Why this priority**: PII 检测是数据安全的高级需求。它依赖分级敏感词体系（PII 规则也需要 level/action 配置），因此在 Phase 4 之后实现。

**Independent Test**: 启用手机号检测，发送包含手机号的请求，验证系统能检测到并根据配置执行脱敏/阻断/记录。

**Acceptance Scenarios**:

1. **Given** 管理员启用了手机号检测（action=mask），**When** 用户 prompt 包含手机号，**Then** 手机号在输入侧被脱敏后发送给上游 LLM；当 LLM 响应中也包含手机号时，输出侧也执行脱敏
2. **Given** 管理员启用了身份证检测（action=block），**When** 用户 prompt 包含身份证号，**Then** 请求被拒绝并记录审计日志
3. **Given** 管理员启用了邮箱检测（action=log），**When** LLM 响应包含邮箱地址，**Then** 响应正常返回但记录审计日志
4. **Given** 管理员为每种 PII 类型配置了不同策略，**When** 请求同时包含多种 PII，**Then** 每种 PII 按其独立策略处理

### Edge Cases

- 流式响应中检测到敏感词时，已发送的 chunk 无法撤回，系统只能记录审计日志；非流式响应可在发送前阻断
- 敏感词规则为空时，系统应跳过检查而非报错
- PII 正则可能误报（如 16 位数字不是银行卡号），管理员可通过禁用特定 PII 类型来控制
- 用户组级别的敏感词规则与全局规则叠加时，组级规则优先
- CSV 导入格式错误时，系统应返回明确的错误信息而非静默失败

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: 系统 MUST 支持对 LLM 响应内容进行敏感词检查（输出侧过滤）
- **FR-002**: 管理员 MUST 能够通过开关独立启用/禁用输入侧和输出侧过滤
- **FR-003**: 系统 MUST 支持三种敏感词级别：block（阻断）、warn（告警）、log（记录）
- **FR-004**: 管理员 MUST 能够为每个敏感词设置级别、类别和适用用户组
- **FR-005**: 系统 MUST 支持通过 CSV 和 TXT 格式批量导入敏感词规则
- **FR-006**: 系统 MUST 支持将敏感词规则导出为 CSV 格式
- **FR-007**: 系统 MUST 支持检测以下 PII 类型：手机号、身份证号、银行卡号、邮箱、IPv4 地址
- **FR-008**: 管理员 MUST 能够为每种 PII 类型独立配置处理策略（mask/block/log）
- **FR-009**: 系统 MUST 在检测到敏感内容时记录审计日志，包含方向（输入/输出）、命中词、处理动作
- **FR-010**: 系统 MUST 保持向后兼容：现有的扁平敏感词列表作为 block 级别规则的后备，与 SensitiveRules 合并使用，不破坏现有配置

### Key Entities

- **SensitiveRule（敏感词规则）**: 代表一条敏感词配置，包含关键词、严重级别（block/warn/log）、类别（政治/色情/暴力/自定义）、适用用户组。唯一性规则：同一用户组内 word 唯一（word + group 联合唯一），不同组可有相同 word 但不同级别
- **PIIConfig（PII 配置）**: 代表一种 PII 类型的检测配置，包含类型（手机号/身份证等）、是否启用、处理策略（mask/block/log）
- **AuditLogDetail（审计日志详情）**: 记录敏感内容检测的详细信息，包含方向、命中词、处理动作、内容预览

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 输出侧过滤启用后，包含敏感词的 LLM 响应 100% 被检测并记录审计日志
- **SC-002**: 分级敏感词规则支持 block/warn/log 三种级别，不同级别请求处理结果符合预期
- **SC-003**: PII 检测对标准格式的手机号、身份证号、银行卡号、邮箱的检出率 ≥ 95%
- **SC-004**: CSV 导入支持 1000+ 条规则的批量操作，完成时间 < 5 秒
- **SC-005**: 敏感词检查对请求延迟的影响 < 10ms（Aho-Corasick + 正则匹配）
- **SC-006**: 现有扁平敏感词列表自动迁移为 block 级别规则，无需人工干预

## Clarifications

### Session 2026-06-02

- Q: 敏感词规则唯一性如何定义？ → A: word + group 联合唯一（同组内 word 唯一，不同组可有同 word 不同级别）
- Q: 流式响应下输出侧过滤如何处理敏感词？ → A: 仅记录审计日志（log-only），不阻断。流式模式下 chunk 已逐步发送给客户端，无法撤回
- Q: 非流式响应下输出侧过滤支持哪些行为？ → A: 支持 block 和 log 两种行为。非流式响应内容在发送前已完整可用，可阻断
- Q: PII 脱敏（mask）在哪些方向执行？ → A: 输入侧和输出侧都执行脱敏（双向保护）。输入侧防止 PII 泄露到上游 LLM，输出侧防止 LLM 响应中的 PII 返回给用户
- Q: 现有扁平敏感词列表如何迁移到分级规则体系？ → A: 扁平列表作为 block 级别规则的后备，与 SensitiveRules 合并使用（向后兼容，不破坏现有配置）

## Assumptions

- Phase 1（审计日志增强）已完成，审计日志写入和前端查询功能可用
- 现有 Aho-Corasick 多模式匹配引擎性能满足要求
- 流式响应模式下，已发送给客户端的 chunk 无法撤回，输出侧过滤仅记录不阻断
- 管理员有权限管理所有敏感词规则和 PII 配置
- PII 检测使用正则匹配，不依赖外部 NLP 服务
- 用户组（group）的概念已存在于系统中，可用于敏感词规则隔离
