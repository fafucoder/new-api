# 渠道兜底策略设计文档

## 1. 概述

### 1.1 需求背景

当模型请求失败时（重试完出错，或上游报错），如果存在兜底渠道，则在最终失败前再请求这个兜底渠道，提高请求成功率和系统可用性。

### 1.2 设计目标

- 在所有常规重试失败后，提供最后一层保障机制
- 兜底渠道独立管理，不参与常规请求
- 支持所有 API 类型（文本生成、图像、音频、embedding 等）
- 按兜底渠道实际价格计费，保证计费准确性
- 完整的日志记录，便于问题排查和监控

## 2. 架构设计

### 2.1 整体架构

兜底策略在现有请求处理流程中增加独立的"兜底阶段"，位于所有常规重试逻辑之后、最终返回错误之前。

**架构分层：**

```
┌─────────────────────────────────────┐
│   请求处理层 (Controller)            │
│   - 协调常规/重试/兜底完整流程        │
│   - 处理计费退款和重算               │
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│   渠道选择层 (Service)                │
│   - 常规渠道选择 (排除兜底)           │
│   - 兜底渠道选择 (仅兜底)             │
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│   数据层 (Model)                     │
│   - is_fallback 标记                 │
│   - 按条件查询兜底/非兜底渠道         │
└─────────────────────────────────────┘
```

### 2.2 请求流程

```
用户请求
  ↓
常规渠道选择（过滤 is_fallback=true）
  ↓
尝试请求
  ↓ 失败
重试逻辑（N次，从非兜底渠道中选择）
  ↓ 全部失败
检查是否存在可用兜底渠道
  ↓ 存在
退还原渠道预扣费
  ↓
按兜底渠道价格重新预扣费
  ↓
执行兜底请求
  ↓ 成功
按实际 token 结算（多退少补）→ 返回成功响应
  ↓ 失败
退还兜底预扣费 → 返回最终错误
```

## 3. 数据模型设计

### 3.1 Channel 表结构变更

**新增字段：**

```go
type Channel struct {
    // ... 现有字段 ...
    IsFallback *bool `json:"is_fallback" gorm:"default:false"` // 是否为兜底渠道
    // ... 现有字段 ...
}
```

**数据库迁移：**

```sql
-- SQLite / MySQL
ALTER TABLE channels ADD COLUMN is_fallback TINYINT(1) DEFAULT 0;

-- PostgreSQL
ALTER TABLE channels ADD COLUMN is_fallback BOOLEAN DEFAULT false;
```

### 3.2 渠道隔离机制

**常规渠道选择：**
- 查询条件：`(is_fallback = false OR is_fallback IS NULL)`
- 保证常规请求只从非兜底渠道中选择

**兜底渠道选择：**
- 查询条件：`is_fallback = true AND status = 1`
- 必须支持请求的模型（严格匹配 models 字段）
- 按优先级（Priority）和权重（Weight）排序

## 4. 兜底执行流程

### 4.1 触发条件

兜底逻辑在以下条件**同时满足**时触发：

1. 所有常规重试已完成且全部失败
2. 存在至少一个可用的兜底渠道
3. 兜底渠道支持请求的模型
4. 兜底渠道未在本次请求中使用过（去重）

### 4.2 执行步骤

**步骤1：准备阶段**
```go
// 记录原始错误
originalError := newAPIError

// 查询兜底渠道
fallbackChannel, err := service.GetFallbackChannel(
    relayInfo.OriginModelName, 
    c.GetStringSlice("use_channel"),
)
if err != nil || fallbackChannel == nil {
    return // 无可用兜底渠道，返回原错误
}

// 设置兜底渠道上下文
middleware.SetupContextForSelectedChannel(c, fallbackChannel, relayInfo.OriginModelName)
```

**步骤2：计费处理**
```go
// 退还原渠道预扣费
if relayInfo.Billing != nil {
    relayInfo.Billing.Refund(c)
    relayInfo.Billing = nil
}

// 重新计算兜底渠道价格（使用原有的 token 估算）
fallbackPriceData, err := helper.ModelPriceHelper(c, relayInfo, 
    relayInfo.EstimatePromptTokens, meta)
if err != nil {
    logger.LogError(c, "fallback price calculation failed")
    return
}

// 按兜底渠道价格预扣费
if !fallbackPriceData.FreeModel {
    if err := service.PreConsumeBilling(c, fallbackPriceData.QuotaToPreConsume, relayInfo); err != nil {
        logger.LogError(c, "fallback pre-consume failed")
        return
    }
}
```

**步骤3：执行请求**
```go
// 重置请求体
bodyStorage, _ := common.GetBodyStorage(c)
c.Request.Body = io.NopCloser(bodyStorage)

// 记录兜底渠道
addUsedChannel(c, fallbackChannel.Id, true)

// 执行兜底请求（根据 relayFormat 调用相应 handler）
switch relayFormat {
case types.RelayFormatOpenAIRealtime:
    newAPIError = relay.WssHelper(c, relayInfo)
case types.RelayFormatClaude:
    newAPIError = relay.ClaudeHelper(c, relayInfo)
case types.RelayFormatGemini:
    newAPIError = geminiRelayHandler(c, relayInfo)
default:
    newAPIError = relayHandler(c, relayInfo)
}
```

**步骤4：结果处理**
```go
if newAPIError == nil {
    // 兜底成功
    logger.LogInfo(c, fmt.Sprintf("兜底请求成功：渠道 #%d，原始错误：%s", 
        fallbackChannel.Id, originalError.Error()))
    relayInfo.LastError = nil
    return // 成功返回
} else {
    // 兜底失败
    logger.LogError(c, fmt.Sprintf("兜底请求失败：渠道 #%d，兜底错误：%s", 
        fallbackChannel.Id, newAPIError.Error()))
    // newAPIError 会在 defer 中自动退款
    // 恢复原始错误供返回
    newAPIError = originalError
}
```

## 5. 计费处理

### 5.1 计费流程

**完整流程：**

1. **PreConsumeBilling**（原渠道，估算 token，原渠道价格）
2. 常规请求/重试（全部失败）
3. **Refund**（退还原渠道预扣费）
4. **ModelPriceHelper**（重新计算兜底渠道价格参数）
5. **PreConsumeBilling**（兜底渠道，估算 token，兜底渠道价格）
6. 兜底请求执行
7. **SettleBilling**（兜底渠道，实际 token，多退少补）

### 5.2 为什么需要重新计费

**原因：**
- 原渠道和兜底渠道可能属于不同分组，分组倍率不同
- 渠道本身的模型倍率配置可能不同
- 渠道的其他价格参数（如按次计费配置）可能不同

**关键点：**
- Token 估算不变（请求内容未变）
- 价格参数按兜底渠道重新计算
- 最终结算使用实际返回的 token 数

### 5.3 边界情况

**情况1：原渠道免费，兜底渠道收费**
- 无需退款，直接预扣兜底渠道费用

**情况2：原渠道收费，兜底渠道免费**
- 退还原渠道费用，跳过兜底预扣

**情况3：余额不足**
- 预扣费失败时，跳过兜底，返回原错误
- 记录日志："fallback pre-consume failed due to insufficient balance"

**情况4：兜底请求失败**
- defer 中的 Refund 自动退还兜底预扣费
- 用户不会为失败的兜底请求付费

## 6. 渠道选择实现

### 6.1 常规渠道选择改造

**文件：** `service/channel.go`

**改造点：** `CacheGetRandomSatisfiedChannel` 函数

```go
// 在查询时添加过滤条件
query = query.Where("(is_fallback = ? OR is_fallback IS NULL)", false)
```

**说明：**
- `is_fallback = false`：显式标记为非兜底
- `is_fallback IS NULL`：兼容旧数据（迁移前的渠道）

### 6.2 兜底渠道选择

**文件：** `service/channel_fallback.go`（新增）

**函数签名：**
```go
func GetFallbackChannel(modelName string, usedChannels []string) (*model.Channel, error)
```

**实现逻辑：**

```go
// 1. 查询条件
db := model.DB.Where("is_fallback = ?", true).
    Where("status = ?", constant.ChannelStatusEnabled)

// 2. 模型匹配（严格匹配）
db = db.Where("FIND_IN_SET(?, models) > 0", modelName)

// 3. 排除已使用的渠道（去重）
if len(usedChannels) > 0 {
    db = db.Where("id NOT IN (?)", usedChannels)
}

// 4. 排序（优先级、权重）
db = db.Order("priority DESC").Order("weight DESC")

// 5. 获取第一个匹配的渠道
var channel model.Channel
if err := db.First(&channel).Error; err != nil {
    return nil, err
}

return &channel, nil
```

### 6.3 模型兼容性

**严格匹配策略：**
- 兜底渠道必须在 `models` 字段中明确包含请求的模型
- 不进行模型映射或自动转换
- 如果兜底渠道不支持该模型，则跳过

**原因：**
- 避免兜底时自动映射导致的意外行为
- 保证兜底响应与预期一致

## 7. 日志与监控

### 7.1 兜底事件日志

**日志级别：** Info / Error

**记录内容：**

```go
// 兜底开始
logger.LogInfo(c, fmt.Sprintf(
    "触发兜底策略：原渠道全部失败（已重试%d次），尝试兜底渠道 #%d (%s)",
    common.RetryTimes, fallbackChannel.Id, fallbackChannel.Name,
))

// 兜底成功
logger.LogInfo(c, fmt.Sprintf(
    "兜底请求成功：渠道 #%d，原始失败原因：%s",
    fallbackChannel.Id, relayInfo.LastError.Error(),
))

// 兜底失败
logger.LogError(c, fmt.Sprintf(
    "兜底请求失败：渠道 #%d，兜底错误：%s，原始错误：%s",
    fallbackChannel.Id, newAPIError.Error(), relayInfo.LastError.Error(),
))
```

### 7.2 渠道使用标记

**在 `use_channel` 数组中标记：**

```go
func addUsedChannel(c *gin.Context, channelId int, isFallback bool) {
    useChannel := c.GetStringSlice("use_channel")
    if isFallback {
        useChannel = append(useChannel, fmt.Sprintf("fallback:%d", channelId))
    } else {
        useChannel = append(useChannel, fmt.Sprintf("%d", channelId))
    }
    c.Set("use_channel", useChannel)
}
```

**日志输出示例：**
```
重试：[123->456->fallback:789]
```

### 7.3 错误日志增强

**在 `admin_info` 中记录兜底信息：**

```go
if isFallbackRequest {
    adminInfo["fallback_triggered"] = true
    adminInfo["original_error"] = originalError.Error()
    adminInfo["original_channels"] = usedChannelsBefore
    adminInfo["fallback_channel_id"] = fallbackChannel.Id
    adminInfo["fallback_channel_name"] = fallbackChannel.Name
}
```

### 7.4 响应头标记（可选）

**在兜底成功的响应中添加自定义头：**

```go
if fallbackSuccess {
    c.Header("X-Fallback-Used", "true")
    c.Header("X-Fallback-Channel", fmt.Sprintf("%d", fallbackChannel.Id))
}
```

**用途：**
- 方便客户端识别是否使用了兜底
- 便于问题排查和统计分析

## 8. 前端配置界面

### 8.1 渠道列表页面

**文件：** `web/classic/src/pages/Channel/ChannelList.jsx`

**UI 改动：**

1. **表格新增列：** "兜底渠道"
   - 显示：🛡️ 图标或 "兜底" 徽章
   - 位置：状态列附近

2. **筛选功能：**
   - 添加下拉选项："全部" / "仅兜底渠道" / "仅常规渠道"

3. **批量操作：**
   - "设为兜底渠道"
   - "取消兜底标记"

### 8.2 渠道编辑表单

**文件：** `web/classic/src/pages/Channel/EditChannel.jsx`

**表单字段：**

```jsx
<Form.Checkbox 
    field="is_fallback"
    label="标记为兜底渠道"
    extraText="兜底渠道仅在所有常规渠道失败后使用，不参与常规请求"
/>
```

**位置：** 渠道状态或优先级字段附近

### 8.3 国际化

**中文（`web/classic/src/i18n/locales/zh.json`）：**
```json
{
  "channel.is_fallback": "兜底渠道",
  "channel.is_fallback_desc": "兜底渠道仅在所有常规渠道失败后使用",
  "channel.set_fallback": "设为兜底渠道",
  "channel.unset_fallback": "取消兜底标记",
  "channel.fallback_badge": "兜底"
}
```

**英文（`web/classic/src/i18n/locales/en.json`）：**
```json
{
  "channel.is_fallback": "Fallback Channel",
  "channel.is_fallback_desc": "Fallback channels are only used when all regular channels fail",
  "channel.set_fallback": "Set as Fallback",
  "channel.unset_fallback": "Remove Fallback Mark",
  "channel.fallback_badge": "Fallback"
}
```

### 8.4 数据验证

**前端提示：**

1. 如果所有渠道都标记为兜底：
   - 警告："建议至少保留一些常规渠道用于正常请求"

2. 如果某个模型没有兜底渠道：
   - 提示："该模型暂无兜底渠道，失败后将直接返回错误"

## 9. 测试与边界情况

### 9.1 核心测试场景

**场景1：正常兜底流程**
- 输入：常规渠道全部失败，存在可用兜底渠道
- 预期：兜底请求成功，返回成功响应，按兜底渠道计费
- 验证：日志正确记录，`use_channel` 包含 `fallback:*`

**场景2：兜底渠道也失败**
- 输入：常规渠道失败，兜底渠道也返回错误
- 预期：返回原始错误，兜底费用已退款
- 验证：用户余额正确，日志记录两次失败

**场景3：无可用兜底渠道**
- 输入：常规渠道失败，没有标记为兜底的渠道
- 预期：直接返回失败，不尝试兜底
- 验证：日志不包含兜底相关信息

**场景4：余额不足**
- 输入：常规渠道失败后退款，余额不足支付兜底费用
- 预期：跳过兜底，返回原始错误
- 验证：日志记录 "fallback pre-consume failed"

**场景5：兜底渠道去重**
- 输入：常规重试已使用过渠道 A，渠道 A 同时标记为兜底
- 预期：兜底阶段跳过渠道 A
- 验证：不会重复请求同一渠道

**场景6：模型不匹配**
- 输入：请求模型 A，兜底渠道只支持模型 B
- 预期：跳过该兜底渠道
- 验证：日志记录 "no fallback channel supports model A"

### 9.2 边界情况处理

**情况1：所有渠道都是兜底渠道**
- 常规请求阶段无可用渠道
- 直接返回 "no available channel" 错误
- 不触发兜底逻辑

**情况2：兜底渠道禁用**
- 查询时自动过滤 `status != 1`
- 如无可用兜底渠道，返回原错误

**情况3：流式响应中断**
- 如果响应已开始发送（部分数据已返回），无法执行兜底
- 处理：仅在响应未发送前失败时触发兜底
- 检测：通过 `c.Writer.Written()` 判断

**情况4：WebSocket 连接**
- WebSocket 连接建立后无法切换渠道
- 处理：仅在连接建立前失败时触发兜底

**情况5：Task API**
- Task API 也需要兜底支持
- 在 `RelayTask` 函数中实现相同的兜底逻辑

### 9.3 性能优化

**缓存策略：**
- 兜底渠道列表可以缓存
- 缓存键：`fallback_channels:{model_name}`
- 过期时间：与普通渠道缓存一致（如 5 分钟）

**异步处理：**
- 兜底日志使用 `gopool.Go` 异步记录
- 不阻塞主请求流程

**超时控制：**
- 兜底请求复用现有的超时配置
- 建议为兜底渠道配置合理的超时时间

### 9.4 监控指标（建议）

**统计维度：**
- 兜底触发次数（按模型、分组）
- 兜底成功率
- 兜底渠道使用分布
- 因兜底节省的失败请求数

**告警规则：**
- 兜底触发率 > 10%（可能表示常规渠道故障）
- 兜底成功率 < 50%（需检查兜底渠道配置）
- 特定兜底渠道使用率过高（负载不均）

## 10. 实现清单

### 10.1 后端改动

**数据库层：**
- [ ] 添加 `is_fallback` 字段迁移
- [ ] 兼容 SQLite、MySQL、PostgreSQL

**模型层（`model/channel.go`）：**
- [ ] Channel 结构体增加 `IsFallback` 字段
- [ ] 更新 JSON 序列化标签

**服务层（`service/`）：**
- [ ] `channel.go`：改造 `CacheGetRandomSatisfiedChannel`，过滤兜底渠道
- [ ] `channel_fallback.go`（新增）：实现 `GetFallbackChannel` 函数
- [ ] `billing.go`：确保兜底场景的计费逻辑正确

**控制器层（`controller/relay.go`）：**
- [ ] `Relay` 函数：在重试循环后添加兜底逻辑
- [ ] `RelayTask` 函数：添加 Task API 兜底逻辑
- [ ] `addUsedChannel` 函数：支持兜底标记

**辅助函数：**
- [ ] `helper/price.go`：确保兜底场景价格计算正确
- [ ] `logger`：添加兜底相关日志格式

### 10.2 前端改动

**渠道列表页（`web/classic/src/pages/Channel/`）：**
- [ ] 表格增加"兜底渠道"列
- [ ] 添加筛选功能
- [ ] 添加批量操作

**渠道编辑页：**
- [ ] 表单增加 `is_fallback` 复选框
- [ ] 添加说明文案

**国际化：**
- [ ] `zh.json`：添加中文翻译
- [ ] `en.json`：添加英文翻译

**API 接口：**
- [ ] `GET /api/channel`：返回 `is_fallback` 字段
- [ ] `POST /api/channel`：支持设置 `is_fallback`
- [ ] `PUT /api/channel/:id`：支持更新 `is_fallback`
- [ ] `PATCH /api/channel/batch`：支持批量设置兜底标记

### 10.3 测试

- [ ] 单元测试：渠道选择逻辑
- [ ] 单元测试：计费重算逻辑
- [ ] 集成测试：完整兜底流程
- [ ] 集成测试：边界情况覆盖
- [ ] 性能测试：兜底对整体延迟的影响

### 10.4 文档

- [ ] API 文档：新增字段说明
- [ ] 用户文档：兜底功能使用指南
- [ ] 运维文档：监控指标和告警配置

## 11. 上线计划

### 11.1 灰度发布

**阶段1：数据库迁移**
- 执行迁移脚本，添加 `is_fallback` 字段
- 所有现有渠道默认 `is_fallback = false`

**阶段2：后端部署**
- 部署包含兜底逻辑的后端代码
- 初期不标记任何渠道为兜底，观察系统稳定性

**阶段3：前端部署**
- 部署前端配置界面
- 管理员可以开始配置兜底渠道

**阶段4：小范围测试**
- 为少量模型/分组配置兜底渠道
- 观察兜底触发率和成功率

**阶段5：全量发布**
- 根据测试结果调整配置
- 为所有关键模型配置兜底渠道

### 11.2 回滚方案

**如果出现问题：**
1. 通过配置取消所有兜底标记（`UPDATE channels SET is_fallback = false`）
2. 兜底逻辑不会被触发，系统恢复到原有行为
3. 无需回滚代码或数据库

### 11.3 监控指标

**上线后重点观察：**
- 整体请求成功率变化
- 兜底触发频率
- 兜底成功率
- P99 延迟变化
- 计费准确性

## 12. 未来扩展

### 12.1 可能的优化方向

**1. 多级兜底**
- 支持配置多个兜底渠道，按优先级依次尝试
- 第一个兜底失败后，尝试第二个兜底

**2. 智能兜底**
- 根据错误类型选择合适的兜底渠道
- 如：超时错误优先选择响应速度快的兜底渠道

**3. 兜底渠道自动发现**
- 系统自动识别高可用性渠道，推荐标记为兜底

**4. 按分组配置兜底**
- 不同分组可以配置不同的兜底渠道池
- 更灵活的兜底策略

### 12.2 注意事项

当前设计遵循 YAGNI 原则，以上扩展功能暂不实现，待实际需求明确后再考虑。

## 13. 总结

本设计实现了一个完整的渠道兜底机制，关键特性包括：

✅ 兜底渠道独立管理，不参与常规请求  
✅ 支持所有 API 类型（文本、图像、音频等）  
✅ 计费准确，按兜底渠道实际价格结算  
✅ 完整的日志和监控支持  
✅ 前端配置界面友好  
✅ 性能影响可控  
✅ 安全的灰度发布和回滚方案  

该设计在保证功能完整性的同时，最小化对现有系统的侵入性，便于维护和扩展。
