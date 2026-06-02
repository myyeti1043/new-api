# Data Model: 敏感词过滤增强（Phase 2-4）

## 实体定义

### SensitiveRule（敏感词规则）

**存储方式**：Options 表，key=`SensitiveRules`，value=JSON 数组

**字段**：

| 字段 | 类型 | 说明 |
|------|------|------|
| word | string | 敏感词（小写，用于 Aho-Corasick 匹配） |
| level | string | 严重级别：`block` / `warn` / `log` |
| category | string | 类别：`politics` / `porn` / `violence` / `pii` / `custom` |
| group | string | 适用用户组，`""` 表示所有组 |

**唯一性**：word + group 联合唯一（同组内 word 唯一，不同组可有同 word 不同级别）

**向后兼容**：现有扁平 `SensitiveWords` 列表作为 block 级别规则的后备，与 `SensitiveRules` 合并使用。

```json
// SensitiveRules JSON 示例
[
  {"word": "敏感词1", "level": "block", "category": "politics", "group": ""},
  {"word": "敏感词2", "level": "warn", "category": "custom", "group": "vip"},
  {"word": "敏感词3", "level": "log", "category": "custom", "group": ""}
]
```

---

### PIIConfig（PII 配置）

**存储方式**：Options 表，每个 PII 类型独立 key

**配置项**：

| Option Key | 类型 | 说明 |
|------------|------|------|
| PIIEnabled | bool | PII 检测总开关 |
| PIIPhoneEnabled | bool | 手机号检测开关 |
| PIIPhoneAction | string | 手机号处理策略：`mask` / `block` / `log` |
| PIIIDCardEnabled | bool | 身份证检测开关 |
| PIIIDCardAction | string | 身份证处理策略 |
| PIIBankCardEnabled | bool | 银行卡检测开关 |
| PIIBankCardAction | string | 银行卡处理策略 |
| PIIEmailEnabled | bool | 邮箱检测开关 |
| PIIEmailAction | string | 邮箱处理策略 |
| PIIIPv4Enabled | bool | IPv4 检测开关 |
| PIIIPv4Action | string | IPv4 处理策略 |

---

### AuditLogDetail（审计日志详情）

**存储方式**：Log 表的 Other 字段（JSON 格式）

**已有定义**（Phase 1）：

| 字段 | 类型 | 说明 |
|------|------|------|
| direction | string | `input` / `output` |
| sensitive_words | string[] | 命中的敏感词 |
| action | string | `blocked` / `replaced` / `logged` |
| content_preview | string | 内容预览（截断+脱敏） |
| rule_level | string | `block` / `warn` / `log` |
| category | string | 规则类别 |

**Phase 2-4 扩展**：无新增字段，复用现有结构。direction 字段新增 `output` 值用于输出侧过滤。

---

### 输出侧过滤配置

**存储方式**：Options 表

| Option Key | 类型 | 说明 |
|------------|------|------|
| CheckSensitiveOnCompletionEnabled | bool | 输出侧过滤开关（默认 false） |

---

## 关系图

```
Options 表
├── SensitiveWords (string) ← 扁平词列表，block 级别后备
├── SensitiveRules (JSON) ← 分级规则列表
├── CheckSensitiveEnabled (bool) ← 总开关
├── CheckSensitiveOnPromptEnabled (bool) ← 输入侧开关
├── CheckSensitiveOnCompletionEnabled (bool) ← 输出侧开关
├── StopOnSensitiveEnabled (bool) ← 阻断开关
├── PIIEnabled (bool) ← PII 总开关
├── PIIPhoneEnabled / PIIPhoneAction
├── PIIIDCardEnabled / PIIIDCardAction
├── PIIBankCardEnabled / PIIBankCardAction
├── PIIEmailEnabled / PIIEmailAction
└── PIIIPv4Enabled / PIIIPv4Action

Log 表
└── type=7 (LogTypeAudit)
    └── Other (JSON) → AuditLogDetail
```

## 状态转换

### 敏感词规则生命周期

```
创建 → 生效（立即，Options 表加载时）
修改 → 生效（Options 同步周期内）
删除 → 失效（Options 同步周期内）
```

### PII 配置生命周期

```
默认关闭 → 管理员开启 → 生效（Options 同步周期内）
管理员关闭 → 失效（Options 同步周期内）
```
