# PII 配置管理 API 合约

## 1. 获取 PII 配置

**GET /api/pii/config**

### 响应（200）

```json
{
  "success": true,
  "message": "",
  "data": {
    "enabled": true,
    "input_action": "mask",
    "output_action": "mask",
    "audit_log_enabled": true,
    "types": {
      "phone":    { "enabled": true,  "action": "mask" },
      "idcard":   { "enabled": true,  "action": "mask" },
      "bankcard": { "enabled": true,  "action": "mask" },
      "email":    { "enabled": false, "action": "log"  },
      "ipv4":     { "enabled": false, "action": "log"  }
    }
  }
}
```

### 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `enabled` | bool | 全局 PII 检测开关 |
| `input_action` | string | 输入端默认动作：mask / block / log |
| `output_action` | string | 输出端默认动作：mask / block / log |
| `audit_log_enabled` | bool | 是否记录审计日志 |
| `types.*.enabled` | bool | 该类型是否启用 |
| `types.*.action` | string | 该类型独立动作（覆盖默认）：mask / block / log |

---

## 2. 更新 PII 配置

**PUT /api/pii/config**

### 请求体

```json
{
  "enabled": true,
  "input_action": "mask",
  "output_action": "block",
  "types": {
    "phone":  { "enabled": true, "action": "mask" },
    "idcard": { "enabled": true, "action": "block" }
  }
}
```

### 响应（200）

```json
{
  "success": true,
  "message": "PII 配置已更新"
}
```

---

## 3. 手动正则测试

**POST /api/pii/test**

### 请求体

```json
{
  "text": "我的手机号是13800138000，身份证号是110101199001011234",
  "types": ["phone", "idcard"]
}
```

### 响应（200）

```json
{
  "success": true,
  "message": "",
  "data": {
    "findings": [
      {
        "type": "phone",
        "start": 6,
        "end": 17,
        "original": "13800138000"
      },
      {
        "type": "idcard",
        "start": 23,
        "end": 41,
        "original": "110101199001011234"
      }
    ],
    "masked_text": "我的手机号是138****8000，身份证号是110101********1234"
  }
}
```

---

## 4. 错误响应

### 400 — 请求参数错误

```json
{
  "success": false,
  "message": "invalid PII type: foo"
}
```

### 401 — 未授权（非管理员）

```json
{
  "success": false,
  "message": "无权访问"
}
```

---

## 5. PII 类型定义

| 类型 | 说明 | 默认正则 | 默认掩码格式 |
|------|------|----------|-------------|
| `phone` | 中国大陆手机号 | `1[3-9]\d{9}` | `138****8000` |
| `idcard` | 18位身份证号 | `[1-9]\d{5}(19\|20)\d{2}(0[1-9]\|1[0-2])(0[1-9]\|[12]\d\|3[01])\d{3}[\dXx]` | `110101********1234` |
| `bankcard` | 银行卡号(16-19位) | `[1-9]\d{15,18}` | `6222 **** **** 1234` |
| `email` | 电子邮箱 | `[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}` | `te***@example.com` |
| `ipv4` | IPv4 地址 | `((25[0-5]\|2[0-4]\d\|[01]?\d\d?)\.){3}(25[0-5]\|2[0-4]\d\|[01]?\d\d?)` | `192.168.*.*` |

## 6. 动作（Action）说明

| 动作 | 输入端行为 | 输出端行为 |
|------|----------|----------|
| `mask` | 将 PII 替换为掩码后转发 | 将响应中的 PII 替换为掩码后返回 |
| `block` | 直接拒绝请求，返回错误 | 阻断响应，返回错误 |
| `log` | 仅记录审计日志，正常转发 | 仅记录审计日志，正常返回 |
