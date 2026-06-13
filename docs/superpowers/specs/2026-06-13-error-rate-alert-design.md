# 错误率监控告警功能设计

## 1. 功能概述

实现实时错误率监控告警功能，当指定时间窗口内的错误率超过阈值时，自动触发告警推送到配置的 webhook 地址。

## 2. 功能特性

### 2.1 核心功能
- ✅ 多时间窗口监控：支持 5分钟、15分钟、30分钟、1小时、6小时
- ✅ 灵活阈值配置：管理员自定义错误率阈值（如 10%）
- ✅ Webhook 推送：支持配置独立的 webhook URL 和密钥
- ✅ 告警抑制：cooldown 机制避免重复告警
- ✅ 状态翻转：错误率恢复后清除告警状态，再次超标时立即触发

### 2.2 监控范围
- **全局监控**：所有请求的错误率
- **渠道维度**：按 channel_id 监控单个渠道
- **标签维度**：按 tag 监控一组渠道

## 3. 数据模型

### 3.1 错误率告警规则表 `error_rate_alert_rules`

```go
type ErrorRateAlertRule struct {
    Id              int     `json:"id" gorm:"primaryKey"`
    Name            string  `json:"name" gorm:"type:varchar(128)"`         // 规则名称
    Scope           string  `json:"scope" gorm:"type:varchar(16)"`         // 监控范围: global/channel/tag
    ScopeValue      string  `json:"scope_value" gorm:"type:varchar(64)"`   // 范围值: ""(global) / channel_id / tag
    TimeWindow      string  `json:"time_window" gorm:"type:varchar(8)"`    // 时间窗口: 5m/15m/30m/1h/6h
    Threshold       float64 `json:"threshold"`                             // 错误率阈值 (0-100, 百分比)
    WebhookURL      string  `json:"webhook_url" gorm:"type:varchar(512)"`
    WebhookSecret   string  `json:"webhook_secret" gorm:"type:varchar(128)"`
    Enabled         bool    `json:"enabled" gorm:"default:true"`
    Remark          string  `json:"remark" gorm:"type:varchar(256)"`
    
    // 运行时状态字段
    LastErrorRate   float64 `json:"last_error_rate"`    // 上次检查的错误率
    LastAlertedAt   int64   `json:"last_alerted_at"`    // 最近一次告警时间
    LastCheckedAt   int64   `json:"last_checked_at"`    // 最近一次检查时间
    AlertState      string  `json:"alert_state" gorm:"type:varchar(16);default:'normal'"` // normal/alerting
    
    CreatedTime     int64   `json:"created_time" gorm:"bigint;autoCreateTime"`
    UpdatedTime     int64   `json:"updated_time" gorm:"bigint;autoUpdateTime"`
}
```

### 3.2 时间窗口配置

```go
var errorRateAlertWindows = map[string]int64{
    "5m":  300,     // 5分钟 = 300秒
    "15m": 900,     // 15分钟 = 900秒
    "30m": 1800,    // 30分钟
    "1h":  3600,    // 1小时
    "6h":  21600,   // 6小时
}
```

## 4. 告警逻辑

### 4.1 扫描流程

```
1. 后台任务周期性扫描 (默认每5分钟)
2. 遍历所有 enabled 规则
3. 对每条规则:
   a. 根据 scope/scope_value/time_window 查询错误率
   b. 判断是否超过阈值
   c. 根据状态翻转 + cooldown 决定是否告警
   d. 发送 webhook (规则自定义 URL 或系统默认)
   e. 更新规则状态
```

### 4.2 状态翻转机制

```
错误率 >= 阈值:
  - 首次超标 → 立即告警，状态 = alerting
  - 持续超标 → 按 cooldown 间隔重复告警
  
错误率 < 阈值:
  - 状态 = normal
  - 清除告警状态，下次超标视为新事件
```

### 4.3 Cooldown 设置

- 默认 cooldown: 60分钟
- 可在系统设置中配置
- 避免同一规则短时间内重复告警

## 5. Webhook 推送格式

### 5.1 请求格式

```json
{
  "type": "error_rate_alert",
  "timestamp": 1717171200,
  "rule": {
    "id": 1,
    "name": "全局错误率监控",
    "scope": "global",
    "scope_value": "",
    "time_window": "5m",
    "threshold": 10.0
  },
  "alert": {
    "error_rate": 15.5,
    "error_count": 31,
    "success_count": 169,
    "total": 200,
    "time_range": {
      "start": 1717170900,
      "end": 1717171200
    }
  },
  "message": "错误率告警: 5分钟内错误率达到 15.5%，超过阈值 10.0%"
}
```

### 5.2 请求头

```
Content-Type: application/json
X-Signature: HMAC-SHA256(body, secret)  // 如果配置了 webhook_secret
User-Agent: new-api-error-rate-alert/1.0
```

## 6. API 接口

### 6.1 管理接口

```
GET    /api/error_rate_alerts          # 列出所有规则
POST   /api/error_rate_alerts          # 创建规则
GET    /api/error_rate_alerts/:id      # 获取规则详情
PUT    /api/error_rate_alerts/:id      # 更新规则
DELETE /api/error_rate_alerts/:id      # 删除规则
POST   /api/error_rate_alerts/:id/test # 测试 webhook 推送
```

### 6.2 系统设置

在 `monitor_setting` 中添加：

```go
AutoErrorRateAlertEnabled      bool    // 启用错误率自动告警
AutoErrorRateAlertMinutes      float64 // 扫描间隔（分钟）
ErrorRateAlertCooldownMinutes  int     // 告警冷却时间（分钟）
```

## 7. 前端界面

### 7.1 规则列表页 `/setting/monitor/error-rate-alerts`

表格列：
- 规则名称
- 监控范围（全局/渠道/标签）
- 时间窗口
- 阈值
- 状态（启用/禁用）
- 最近错误率
- 最近告警时间
- 操作（编辑/删除/测试）

### 7.2 创建/编辑规则表单

字段：
- 规则名称（必填）
- 监控范围（单选：全局/渠道/标签）
- 范围值（渠道下拉/标签下拉）
- 时间窗口（单选：5m/15m/30m/1h/6h）
- 错误率阈值（数字输入，百分比）
- Webhook URL（可选，留空使用系统默认）
- Webhook Secret（可选）
- 启用状态（开关）
- 备注

## 8. 实现步骤

### Phase 1: 后端基础
1. 创建数据模型 `model/error_rate_alert.go`
2. 数据库迁移
3. CRUD API `controller/error_rate_alert.go`
4. 错误率查询服务（复用 `model.QueryErrorRate`）

### Phase 2: 告警服务
1. 后台扫描任务 `service/error_rate_alert.go`
2. Webhook 推送（复用 `service/webhook`）
3. 系统设置扩展

### Phase 3: 前端界面
1. 规则管理页面
2. 创建/编辑表单
3. 测试功能
4. 国际化

## 9. 配置示例

### 9.1 全局监控

```json
{
  "name": "全局5分钟错误率",
  "scope": "global",
  "scope_value": "",
  "time_window": "5m",
  "threshold": 10.0,
  "webhook_url": "https://example.com/webhook",
  "enabled": true
}
```

### 9.2 渠道监控

```json
{
  "name": "OpenAI渠道错误率",
  "scope": "channel",
  "scope_value": "123",
  "time_window": "15m",
  "threshold": 5.0,
  "enabled": true
}
```

### 9.3 标签监控

```json
{
  "name": "生产环境渠道组",
  "scope": "tag",
  "scope_value": "prod",
  "time_window": "30m",
  "threshold": 8.0,
  "enabled": true
}
```

## 10. 注意事项

1. **性能考虑**：错误率查询复用 `model.QueryErrorRate`，已有索引优化
2. **数据一致性**：告警基于 logs 表实时数据，无需额外聚合
3. **告警spam控制**：cooldown + 状态翻转机制
4. **兼容性**：webhook 格式兼容主流告警平台（企业微信、钉钉、Slack）
5. **权限控制**：仅管理员可配置规则

## 11. 后续扩展

- [ ] 恢复通知：错误率降低到阈值以下时发送恢复通知
- [ ] 多级阈值：warning(5%) / critical(10%)
- [ ] 邮件通知：除 webhook 外支持邮件推送
- [ ] 告警历史：记录所有告警事件到独立表
- [ ] 图表展示：错误率趋势图 + 告警标记点
