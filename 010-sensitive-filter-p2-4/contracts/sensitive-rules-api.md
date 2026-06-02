# API Contract: 敏感词规则管理

**Base Path**: `/api/sensitive`
**Auth**: Admin (管理员认证)

---

## GET /api/sensitive/rules

获取所有敏感词规则。

**Query Parameters**:

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| level | string | 否 | 按级别筛选：`block` / `warn` / `log` |
| category | string | 否 | 按类别筛选 |
| group | string | 否 | 按用户组筛选 |
| page | int | 否 | 页码，默认 1 |
| per_page | int | 否 | 每页数量，默认 20 |

**Response** (200):

```json
{
  "success": true,
  "data": {
    "rules": [
      {
        "word": "敏感词1",
        "level": "block",
        "category": "politics",
        "group": ""
      }
    ],
    "total": 100,
    "page": 1,
    "per_page": 20
  }
}
```

---

## POST /api/sensitive/rules

添加单条敏感词规则。

**Request Body**:

```json
{
  "word": "新敏感词",
  "level": "warn",
  "category": "custom",
  "group": ""
}
```

**Validation**:
- `word` 必填，不能为空
- `level` 必填，必须是 `block` / `warn` / `log` 之一
- `category` 可选，默认 `custom`
- `group` 可选，默认 `""`（所有组）
- word + group 联合唯一，重复返回错误

**Response** (200):

```json
{
  "success": true,
  "message": "规则添加成功"
}
```

**Error Response** (400):

```json
{
  "success": false,
  "message": "该用户组下已存在相同的敏感词"
}
```

---

## PUT /api/sensitive/rules

更新敏感词规则。

**Request Body**:

```json
{
  "word": "敏感词1",
  "level": "log",
  "category": "custom",
  "group": ""
}
```

**Response** (200):

```json
{
  "success": true,
  "message": "规则更新成功"
}
```

---

## DELETE /api/sensitive/rules

删除敏感词规则。

**Request Body**:

```json
{
  "word": "敏感词1",
  "group": ""
}
```

**Response** (200):

```json
{
  "success": true,
  "message": "规则删除成功"
}
```

---

## POST /api/sensitive/rules/import

批量导入敏感词规则。

**Request Body** (multipart/form-data):

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file | file | 是 | CSV 或 TXT 文件 |
| format | string | 是 | `csv` 或 `txt` |
| default_level | string | 否 | TXT 导入时的默认级别，默认 `block` |
| default_category | string | 否 | TXT 导入时的默认类别，默认 `custom` |
| default_group | string | 否 | TXT 导入时的默认用户组，默认 `""` |

**CSV 格式**:

```csv
word,level,category,group
敏感词1,block,politics,
敏感词2,warn,custom,vip
```

**TXT 格式**（每行一个词）:

```
敏感词1
敏感词2
敏感词3
```

**Response** (200):

```json
{
  "success": true,
  "data": {
    "imported": 50,
    "skipped": 2,
    "errors": ["第3行: 格式错误"]
  }
}
```

---

## GET /api/sensitive/rules/export

导出敏感词规则为 CSV。

**Query Parameters**:

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| level | string | 否 | 按级别筛选 |
| category | string | 否 | 按类别筛选 |
| group | string | 否 | 按用户组筛选 |

**Response** (200): CSV 文件下载

```csv
word,level,category,group
敏感词1,block,politics,
敏感词2,warn,custom,vip
```

---

## 错误码

| HTTP Status | 说明 |
|-------------|------|
| 200 | 成功 |
| 400 | 请求参数错误 |
| 401 | 未认证 |
| 403 | 非管理员 |
| 500 | 服务器内部错误 |
