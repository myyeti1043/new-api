# Tasks: 敏感词分级过滤 + PII 检测

**功能**: 010-sensitive-filter-p2-4
**规格**: 010-sensitive-filter-p2-4/spec.md
**计划**: 010-sensitive-filter-p2-4/plan.md

---

## Phase 1: Setup（项目初始化）

### 目标
建立项目目录结构和审计日志基础设施。

- [x] T001 在 service/ 目录下创建 pii.go 文件骨架（service/pii.go）
- [x] T002 创建 internal/static/builtin_words.txt 内置敏感词库占位文件（internal/static/builtin_words.txt）
- [x] T003 创建 010-sensitive-filter-p2-4/contracts/p pii-config-api.md PII 配置 API 合约文档（010-sensitive-filter-p2-4/contracts/pii-config-api.md）

---

## Phase 2: Foundational（基础设施 — 阻塞所有用户故事）

### 目标
所有用户故事共享的基础设施。审计日志（Phase 1）已就绪，此处仅补充缺失的共享组件。

### 阻塞原因
US2（分级敏感词）的 SensitiveRule 结构被 US3（PII 检测）依赖；output-side 过滤逻辑被 US1/US2/US3 共用。

- [x] T004 实现内置敏感词库加载逻辑：go:embed builtin_words.txt，启动时解析并合并到 SensitiveWords（setting/sensitive.go + internal/static/builtin_words.go）
- [x] T005 在 RelayInfo 中添加 OutputResponseText strings.Builder 字段，用于各 handler 累积响应文本（relay/common/relay_info.go）

---

## Phase 3: User Story 1 — 输出端内容过滤（P1）

**故事目标**: 当 LLM 响应包含敏感词时，系统执行日志记录或阻断，确保出口合规。
**影响范围**: 7 个 relay handler（OpenAI/Claude/Gemini/Responses × stream/non-stream）
**独立测试**: 开启 Completion 过滤 → 调用含敏感词的 completion → 验证审计日志记录 + 非流式阻断

- [x] T006 [US1] 新增 Setting 项 CheckSensitiveOnCompletionEnabled，默认 false，在 InitOptionMap 和 updateOptionMap 中注册（setting/option.go:340-353）
- [x] T007 [US1] 新增 shouldCheckCompletionSensitive() 函数，检查 CheckSensitiveOnCompletionEnabled && CheckSensitiveEnabled（setting/sensitive.go）
- [x] T008 [US1] [P] OpenAI handler：在 OaiStreamHandler.DoResponse 末尾将 responseTextBuilder 写入 relayInfo.OutputResponseText（relay/channel/openai/relay-openai.go:106-190）
- [x] T009 [US1] [P] OpenAI 非流式 handler：在 OpenaiHandler 末尾将 simpleResponse.Choices[].Message.StringContent() 写入 relayInfo.OutputResponseText（relay/channel/openai/relay-openai.go:192-230）
- [x] T010 [US1] [P] Claude handler：在 ClaudeHelper 末尾将完整响应写入 relayInfo.OutputResponseText（relay/channel/claude/relay-claude.go）
- [x] T011 [US1] [P] Gemini handler：在 geminiRelayHandler 末尾将完整响应写入 relayInfo.OutputResponseText（relay/channel/gemini/relay-gemini.go）
- [x] T012 [US1] [P] Responses handler：在 responsesRelayHandler 末尾将完整响应写入 relayInfo.OutputResponseText（relay/channel/responses/relay-responses.go）
- [x] T013 [US1] 在 controller/relay.go 成功返回点（line ~257）注入输出端检查：调用 service.CheckSensitiveText，命中新 sensitiveAction=output，写入审计日志，非流式且 action=block 时返回错误响应（controller/relay.go）
- [x] T014 [US1] 前端新增 CheckSensitiveOnCompletionEnabled 开关，放在 CheckSensitiveOnPromptEnabled 下方（web/default/src/features/system-settings/request-limits/sensitive-words-section.tsx）
- [x] T015 [US1] 验证：开启 Completion 过滤 → 调用非流式 API 含敏感词内容 → 确认返回错误 + 审计日志记录

---

## Phase 4: User Story 2 — 分级敏感词系统（P2）

**故事目标**: 支持 block/warn/log 三级敏感词规则、多词库、分组隔离、CSV/TXT 导入导出。
**独立测试**: 创建 block+warn+log 规则 → 分别触发 → 验证 block 阻断、warn/log 不阻断但记录审计日志。

- [x] T016 [US2] 定义 SensitiveRule 结构体 {Word, Level, Category, Group} + SensitiveRuleLevel 类型常量（model/sensitive.go 或 model/sensitive_rule.go）
- [x] T017 [US2] 实现 sensitive-rules options 加载/保存：SensitiveRulesToOptionsJson / SensitiveRulesFromOptionsJson（setting/sensitive.go）
- [x] T018 [US2] 在 InitOptionMap / updateOptionMap 中注册 SensitiveRules key（model/option.go:340-353）
- [x] T019 [US2] 实现 CheckSensitiveMessagesWithLevel()：返回命中规则列表（含 Level），支持 group 过滤（service/sensitive.go）
- [x] T020 [US2] 实现 CSV/TXT 导入导出：ExportRulesToCSV / ImportRulesFromCSV / ImportRulesFromTXT（service/sensitive.go 或 service/sensitive_rule_io.go）
- [x] T021 [US2] 实现 SensitiveRule CRUD API：GET/POST/PUT/DELETE /api/sensitive/rules + POST import + GET export（controller/sensitive_rule.go，路由注册在 router/api-router.go）
- [x] T022 [US2] 修改 service.CheckSensitiveMessages() 返回 Level，controller/relay.go 中 action 根据 Level 决定：block 阻断、warn/log 仅记录审计（controller/relay.go + service/sensitive.go）
- [x] T023 [US2] 前端敏感词规则编辑器：列表展示 + 新建/编辑弹窗 + 导入导出按钮（web/default/src/pages/Setting/SensitiveRule/ 或 web/default/src/components/sensitive-rule-editor.tsx）
- [x] T024 [US2] 向后兼容：将旧 SensitiveWords 扁平列表作为 block 级规则合并到 CheckSensitiveMessagesWithLevel 结果（service/sensitive.go）
- [x] T025 [US2] 验证：创建 block/warn/log 各一条规则 → 触发 → 确认行为正确 + 审计日志 Level 字段正确

---

## Phase 5: User Story 3 — PII 检测引擎（P3）

**故事目标**: 检测手机号、身份证号、银行卡号、邮箱、IPv4 地址，支持 mask/block/log 动作。
**独立测试**: 输入含手机号文本 → 确认脱敏后传递/阻断/审计日志记录。

- [x] T026 [US3] 实现 PII 正则模式：手机号（1 开头 11 位）、身份证号（18 位）、银行卡号（16-19 位）、邮箱、IPv4（setting/pii.go 或 service/pii.go）
- [x] T027 [US3] 实现 PIICOnfig options 加载/保存 + InitOptionMap 注册（setting/pii.go + model/option.go）
- [x] T028 [US3] 实现 CheckPIIText()：返回 []PIIFinding{Type, Start, End, Original}，支持按 Type 过滤（service/pii.go）
- [x] T029 [US3] 实现 MaskPIIText()：将命中的 PII 替换为 *** 或 [手机号已脱敏]（service/pii.go）
- [x] T030 [US3] 在 controller/relay.go 注入双向 PII 检查：输入端（prompt）+ 输出端（completion），action=mask 时调用 MaskPIIText，action=block 时阻断，记录审计日志（controller/relay.go）
- [x] T031 [US3] 前端 PII 配置面板：启用开关 + 每种 PII 类型独立开关 + 默认动作选择 + 自定义正则输入（web/default/src/pages/Setting/PII/ 或 web/default/src/components/pii-config.tsx）
- [x] T032 [US3] 验证：输入含手机号+身份证文本 → 确认脱敏输出正确 + 审计日志记录

---

## Final Phase: Polish & Cross-Cutting Concerns

- [x] T033 更新 quickstart.md 文档，补充 PII 检测使用示例（010-sensitive-filter-p2-4/quickstart.md）
- [x] T034 E2E 集成测试：分级敏感词（block/warn/log）+ PII 检测（mask/block/log）+ 输出端过滤，全流程验证

---

## Dependencies

```
Phase 1 (Setup)
  └─→ Phase 2 (Foundational) — T004, T005
        ├─→ Phase 3 (US1: Output Filtering) — T006-T015
        └─→ Phase 4 (US2: Tiered Rules) — T016-T025
              └─→ Phase 5 (US3: PII Detection) — T026-T032
                    └─→ Final Phase (Polish) — T033-T034
```

**关键依赖**:
- Phase 3 (US1) 和 Phase 4 (US2) **可并行执行**，均仅依赖 Phase 2
- Phase 5 (US3) **必须等待** Phase 4 完成（PII 规则复用 SensitiveRule 结构）
- T022（level-aware action）在 Phase 4 内，但影响 Phase 3 的输出端检查逻辑

---

## Parallel Execution Opportunities

### Phase 3 (US1) 内部并行
```
T008 | T009 | T010 | T011 | T012  ← 5 个 handler 改造可并行（不同文件）
T006 + T007 可并行（不同文件）
```

### Phase 4 (US2) 内部并行
```
T016 | T017 | T018  ← 数据结构 + options 加载可并行
T020 | T021  ← 导入导出 + API 可并行
```

### Phase 3 ∥ Phase 4 跨故事并行
```
Phase 3 (T006-T015) ∥ Phase 4 (T016-T025)  ← 无共享文件，可完全并行
```

---

## Implementation Strategy

### MVP 范围（User Story 1 优先）
**输出端内容过滤**是最小可用增量：
- 仅需 T006-T015（10 个任务）
- 改动范围：1 个 Setting 项 + 7 个 handler 各加 1 行 + relay.go 注入 + 前端开关
- 验证简单：开启过滤 → 调用 API → 检查日志/错误响应

### 增量交付顺序
1. **MVP**: Phase 3 (US1) — 输出端过滤可用
2. **增量 1**: Phase 4 (US2) — 分级规则系统，替换扁平列表
3. **增量 2**: Phase 5 (US3) — PII 检测引擎
4. **收尾**: Final Phase — 文档 + E2E 测试

### 风险缓解
- **流式响应无法阻断**: T013 明确仅对 non-stream 生效，stream 仅 log
- **旧版 SensitiveWords 兼容**: T024 合并逻辑确保升级无感
- **PII 误报**: T031 前端支持自定义正则 + 每类独立开关
