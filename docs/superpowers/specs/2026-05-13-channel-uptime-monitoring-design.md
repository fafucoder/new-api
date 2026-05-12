# 渠道 Uptime 监控（服务状态页面）— 设计文档

- 日期：2026-05-13
- 范围：仅 `web/classic` 前端 + 后端
- 入口：系统管理 → 运营设置 → 侧边栏管理 → 控制台区域 → "服务状态"开关；启用后侧边栏出现"服务状态"菜单项，路由 `/console/status`

## 1. 背景与目标

`web/classic` 已存在的 `pages/Status/index.jsx` 当前仅渲染 mock 数据；`SettingsSidebarModulesAdmin.jsx` 已新增 `console.status` 开关，但页面没有真实数据来源。

本设计将"服务状态"页面真实化：以现有 `AutomaticallyTestChannels` 自动渠道测试循环为驱动，新增渠道 Uptime 历史记录表，提供管理员/普通用户两种视图（普通用户脱敏聚合）。

## 2. 设计决策（已确认）

| 决策点 | 选择 |
|---|---|
| 前端范围 | 仅 `web/classic`，不动 `web/default` |
| 数据来源 | 内置 DB 表，存储周期测试历史 |
| 触发机制 | 复用现有 `AutomaticallyTestChannels`，在测试循环中追加历史写入 |
| 保留期 | 硬编码 7 天，每日 cron 清理 |
| 聚合 | 实时聚合（查询时计算） |
| 可见性 | 管理员看渠道级；普通用户看按 `channel_type` 聚合的脱敏视图 |
| 手动单点测试 | 不计入历史，避免污染可用率 |

## 3. 数据模型

新增表 `channel_uptime_records`，通过 GORM `AutoMigrate` 在 `model/main.go` 中注册。

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | int PK auto | 主键 |
| `channel_id` | int, index | 渠道 ID |
| `channel_type` | int, index | 渠道类型快照（避免 join，渠道删除后仍可按类型聚合） |
| `status` | int | `1=success, 2=failure` |
| `response_time_ms` | int | 本次测试延迟（毫秒） |
| `error_message` | varchar(512) | 失败时错误简述；成功为空字符串 |
| `created_time` | int64 bigint, index | Unix 秒 |

**索引**：
- 复合索引 `(channel_id, created_time DESC)` — 加速"按渠道取最近 N 条"。
- 单独索引 `created_time` — 加速清理 SQL。

**跨库兼容（Rule 2）**：
- 全程 GORM 抽象，无 DB 特化语法。
- 清理使用 `DELETE FROM channel_uptime_records WHERE created_time < ?`，三库通用。
- `error_message` 用 `varchar(512)` 限长，避免无限增长；写入前在 Go 侧截断到 500 字符。

**文件**：`model/channel_uptime.go`。

## 4. 后端：检测与写入

### 4.1 写入点

在 `controller/channel-test.go` 的 `testAllChannels` 循环中，每次 `testChannel(...)` 之后、`channel.UpdateResponseTime(milliseconds)` 附近，追加一次异步写入：

```go
gopool.Go(func() {
    record := &model.ChannelUptimeRecord{
        ChannelId:      channel.Id,
        ChannelType:    channel.Type,
        ResponseTimeMs: int(milliseconds),
        CreatedTime:    common.GetTimestamp(),
    }
    if result.newAPIError == nil {
        record.Status = model.ChannelUptimeStatusSuccess // 1
    } else {
        record.Status = model.ChannelUptimeStatusFailure // 2
        record.ErrorMessage = truncate(result.newAPIError.Error(), 500)
    }
    if err := model.RecordChannelUptime(record); err != nil {
        common.SysError("record channel uptime failed: " + err.Error())
    }
})
```

约束：
- 跳过 `ChannelStatusManuallyDisabled`（现循环已跳过）。
- 写入失败仅 `SysError` 日志，不回滚或重试。
- 手动 `TestChannel` handler **不写入**。

### 4.2 清理任务

新增 `service/channel_uptime_cleanup.go`，仿 `AutomaticallyTestChannels` 的 master-node-only 模式：

```go
var autoCleanupOnce sync.Once

func AutomaticallyCleanupUptimeRecords() {
    if !common.IsMasterNode {
        return
    }
    autoCleanupOnce.Do(func() {
        ticker := time.NewTicker(24 * time.Hour)
        // 启动后立即执行一次，再进入 ticker 循环
        _ = model.CleanupExpiredUptimeRecords(7 * 24 * time.Hour)
        for range ticker.C {
            _ = model.CleanupExpiredUptimeRecords(7 * 24 * time.Hour)
        }
    })
}
```

在 `main.go` 现有 `go controller.AutomaticallyTestChannels()` 附近添加 `go service.AutomaticallyCleanupUptimeRecords()`。

### 4.3 频率/开关

完全复用现有运营设置：
- `AutoTestChannelEnabled` — 关闭即停止写入新记录。
- `AutoTestChannelMinutes` — 决定写入频率。

未启用自动测试时不产生新记录；管理员页面渠道状态在数小时后落入 `unknown`，普通用户页面显示"暂无数据"。

## 5. 后端：API 与聚合

### 5.1 路由

```
GET /api/uptime/status
```

注册于 `router/api-router.go` 的认证组下，使用 `middleware.UserAuth()`。Controller 内通过 `c.GetInt("role")` 判断是否管理员决定返回粒度。

### 5.2 管理员视图（`role >= common.RoleAdminUser`）

```json
{
  "success": true,
  "data": {
    "view": "admin",
    "services": [
      {
        "id": 12,
        "name": "Azure-East",
        "type": 8,
        "type_name": "Azure OpenAI",
        "status": "normal",
        "latency_ms": 432,
        "last_check": 1747107948,
        "error": "",
        "history": [1, 1, 0, 1, 1, 1, 1, 1, 1, 1],
        "uptime_24h": 98.5
      }
    ],
    "updated_at": 1747107948
  }
}
```

数据来源：
- 列出所有 `status != ChannelStatusManuallyDisabled` 的渠道。
- 对每个 `channel_id` 取最近 10 条 `channel_uptime_records`（`ORDER BY created_time DESC LIMIT 10`）→ 最新状态、最新延迟、`history`（按时间倒序成 1/0 数组）。
- 取最近 24h 记录计算 `uptime_24h`：`SUM(status=1) / COUNT(*) * 100`。
- 无任何记录的渠道：`status="unknown"`、`history=[]`、`uptime_24h=null`、`latency_ms=0`、`last_check=0`。
- `status` 字段语义：
  - `normal` — 最近一次记录 `status=1`
  - `error` — 最近一次记录 `status=2`
  - `unknown` — 无记录

### 5.3 普通用户视图（脱敏）

```json
{
  "success": true,
  "data": {
    "view": "public",
    "services": [
      {
        "type": 8,
        "type_name": "Azure OpenAI",
        "status": "normal",
        "uptime_24h": 99.1,
        "history": [1, 1, 1, 1, 1, 0, 1, 1, 1, 1]
      }
    ],
    "updated_at": 1747107948
  }
}
```

聚合规则：
- 仅展示**至少有一个启用中（非手动禁用）渠道**的 `channel_type`。
- **不返回**：渠道 id、名称、错误信息、具体延迟、渠道数量。
- `status` 判定（取该类型所有渠道的"最近一次记录"）：
  - 全部 `success` → `normal`
  - 混合 → `degraded`
  - 全部 `failure` → `error`
  - 全部无记录 → `unknown`
- `uptime_24h`：该类型所有渠道最近 24h 内所有记录的 `SUM(status=1) / COUNT(*) * 100`（按记录数加权）。
- `history`：把最近 24h 切成 10 个等长时间桶（每桶 ~2.4h），每桶内：
  - 若任一渠道在该桶内有 `success` 记录 → `1`
  - 否则若该桶内有任何记录（全部 failure） → `0`
  - 若该桶内无任何记录 → `-1`（前端渲染灰色"无数据"）

### 5.4 `type_name` 映射

新增 helper `model.GetChannelTypeName(typeId int) string`（或在 `constant/` 下扩展现有 type 表）。优先复用渠道管理 UI 已有的中文映射。若映射缺失则回落到 `"渠道类型 #{n}"`。

实现时需要先检查仓库中是否已有 `channel_type → display_name` 的现成映射（推测在 `constant/channel_type.go` 或前端 `web/classic` 中），有则复用，无则在 `constant/` 新增。

## 6. 前端：`web/classic`

### 6.1 文件改动

| 文件 | 改动 |
|---|---|
| `src/pages/Status/index.jsx` | 删除 `mockServices`，改为调用 `/api/uptime/status`；根据 `data.view` 渲染两套布局 |
| `src/components/layout/SiderBar.jsx` | 修正未提交 diff 中"服务状态"项的 `to: '/status'` → 实际应当走 `routerMap.status`（已为 `/console/status`），与 `App.jsx` 路由一致 |
| `src/i18n/locales/{zh,en,fr,ru,ja,vi,zh-CN,zh-TW}.json` | 补齐缺失的 i18n key |

### 6.2 Status 页面行为

1. 挂载时调用 `API.get('/api/uptime/status')`；每 30s 自动刷新；"立即刷新"按钮强制刷新。
2. `data.view === 'admin'`：
   - 保留现有 UI（渠道名、状态徽章、延迟、上次检查、错误信息、最近 10 圆点）。
   - 新增"24h 可用率"行项，与"延迟""上次检查"并列。
3. `data.view === 'public'`：
   - 卡片只展示 `type_name`、聚合状态徽章、24h 可用率、10 桶 history。
   - **隐藏** 服务详情区（延迟、上次检查）、错误信息区。
4. `history` 数组兼容三态：`1`/`0`/`-1`；新增 `.history-dot.unknown { background-color: var(--semi-color-disabled-bg); }`。
5. 顶部 `stats` 卡片（正常/异常/故障/服务商总数）：
   - admin：按渠道数。
   - public：按类型数（"服务商"语义自然对齐）。
6. 加载失败：渲染空状态卡片"暂无数据"，不弹错误 toast。

### 6.3 i18n key

需补齐的 key（以 `web/classic/src/i18n/locales/en.json` 为基准）：

```
服务状态
实时查看各服务的可用性、延迟与最近事件。
立即刷新
正常
异常
故障
服务商
更新于
延迟
上次检查
最近检查
24小时可用率
暂无数据
```

git status 显示 locale 文件已被修改——同步脚本会跳过已存在的 key，不会重复。

### 6.4 不改动

- `SettingsSidebarModulesAdmin.jsx`（`console.status` 开关已就位）。
- 现有 Status 页面的 CSS（仅新增 `.history-dot.unknown` 一条样式）。
- `web/default` 任何文件。

## 7. 安全与隐私

- 普通用户视图脱敏：**绝不返回**渠道 id/name/key/具体错误/具体延迟。Controller 层应严格按 role 分支返回结构体；不要复用同一结构再"删字段"，防止意外泄露。
- 前端 toggle（`console.status`）只控菜单可见性。直达 URL `/console/status` 仍可访问——这与现有 sidebar modules 行为一致；不要在后端 API 上做额外的"toggle 关闭则拒绝"逻辑，避免和管理员预期不符。

## 8. 测试与验收

### 8.1 后端单元测试

`model/channel_uptime_test.go`（或 `controller/channel_uptime_test.go`）：

1. `RecordChannelUptime` 在成功/失败两种入参下写入字段正确。
2. `CleanupExpiredUptimeRecords(7 * 24 * time.Hour)` 只删除 7 天前的记录。
3. 查询函数（admin 视图）：
   - 单个渠道有 >10 条记录 → 返回最近 10 条且倒序。
   - 24h 成功率：构造已知比例数据验证。
   - 无任何记录 → `status="unknown"`，`uptime_24h=null`。
4. 查询函数（public 视图）：
   - 同一 `channel_type` 下多渠道最近一次结果：全成功→`normal`、混合→`degraded`、全失败→`error`、全无→`unknown`。
   - 10 桶时间分布：全空、部分空、全满三种情况。

### 8.2 跨库手测

- SQLite（默认）：AutoMigrate + 写入 + 清理走通。
- MySQL / PostgreSQL：迁移 DDL 通过；清理 SQL 用纯标准 `DELETE ... WHERE created_time < ?`，无需 DB 特化。

### 8.3 集成手测

1. 启用自动渠道测试（运营设置 → 监控设置 → 自动测试），间隔设为 1 分钟。
2. 等 5 分钟后查 `channel_uptime_records` 表应有记录。
3. 管理员访问 `/console/status`：见每个未手动禁用渠道、最新延迟、最近 10 圆点、24h 可用率。
4. 普通用户登录访问同 URL：见按类型聚合的卡片，**不含任何渠道名/错误/延迟**——脱敏关键点必须人肉验证。
5. 关掉自动测试 30 分钟后刷新：管理员页面渠道状态 → `unknown`；普通用户类型卡显示"暂无数据"。
6. 运营设置里关闭"服务状态"开关：普通用户菜单项消失；直达 URL 仍可访问（与现有 sidebar modules 行为一致）。

### 8.4 回归点

- `testAllChannels` 的 Uptime 写入失败**不能**影响渠道自动禁用/启用流程（异步 + 错误吞掉，仅日志）。
- `model/main.go` 的 AutoMigrate 列表里必须加入 `ChannelUptimeRecord`。
- 手动 `TestChannel` 路径不写入 Uptime 记录。

## 9. 不在本次范围

- `web/default` 的对应实现（后续单独立项）。
- 保留期可配置化（当前硬编码 7 天）。
- 按 (channel, model) 细粒度 Uptime（当前按渠道）。
- 历史趋势图表（折线、热力图等）。
- 报警/通知（可用率低于阈值时推送）。

## 10. 文件清单

**新增**：
- `model/channel_uptime.go`
- `controller/channel_uptime.go`
- `service/channel_uptime_cleanup.go`
- `model/channel_uptime_test.go`（或 `controller/channel_uptime_test.go`）

**修改**：
- `model/main.go` — AutoMigrate 注册新表
- `controller/channel-test.go` — `testAllChannels` 循环里加写入调用
- `router/api-router.go` — 新增 `GET /api/uptime/status`
- `main.go` — 启动清理 goroutine
- `constant/channel_type.go`（若需新增 type→name helper）
- `web/classic/src/pages/Status/index.jsx` — 真实化
- `web/classic/src/components/layout/SiderBar.jsx` — 修正 `to` 字段
- `web/classic/src/i18n/locales/*.json` — 补齐 i18n key

**保护信息**：本设计不改动任何 QuantumNous / new-api 相关品牌、版权、模块路径或元数据（Rule 5）。
