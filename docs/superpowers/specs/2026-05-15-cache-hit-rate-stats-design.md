# Cache Hit Rate Statistics — Design

- Date: 2026-05-15
- Status: Approved (pending user review of this spec)
- Owner: TBD

## 1. Background & Goals

new-api 已记录每次请求的 `cache_read` / `input` / `cache_write` token，但只散落在 `logs.Other` JSON blob 中，无法快速做"按渠道/按模型"或"用户今日/累计命中率"维度的查询。

本设计新增独立聚合表与配套接口/前端，使：

1. 管理员能从侧边栏进入"缓存命中率"页，看渠道/模型粒度的缓存利用情况，支持时间区间过滤。
2. 普通用户能在数据看板看到自己今日与历史累计的缓存命中率。
3. 系统设置 → 侧边栏管理 → 管理员区域可由 root 控制上述管理员入口的可见性。

**命中率公式**（与样例一致）：
```
hit_rate = cache_read / (cache_read + input + cache_write)
```
其中 `input` 为已扣除 cache_read 的净 prompt tokens（项目现有约定，见 `service/text_quota.go:200`）。

## 2. Non-Goals

- 不替代现有 `quota_data` 数据看板；不动既有计费/配额逻辑。
- 不存"命中率"派生列，避免聚合时的"率求平均"陷阱，命中率一律由查询时现算。
- 不为 image/video 等 task 计费场景统计缓存（其链路无 cache 字段）。
- 不做按 IP / 按地区等其他维度。

## 3. Data Model

### 3.1 New Table `cache_hit_stats`

| 字段 | 类型 | 说明 / 索引 |
|---|---|---|
| `id` | int PK | GORM 自增 |
| `user_id` | int | `idx_chs_user_hour(priority:1)` |
| `channel_id` | int | `idx_chs_channel_hour(priority:1)` |
| `model_name` | varchar(128) |  |
| `hour_bucket` | bigint | unix 秒，整点；`idx_chs_user_hour(priority:2,sort:desc)`、`idx_chs_channel_hour(priority:2,sort:desc)` |
| `cache_read_tokens` | bigint default 0 |  |
| `input_tokens` | bigint default 0 | 等于 `logs.prompt_tokens`（已扣 cache_read） |
| `cache_write_tokens` | bigint default 0 |  |
| `request_count` | bigint default 0 |  |

**唯一约束**：`uniq_chs_user_channel_model_hour(user_id, channel_id, model_name, hour_bucket)` —— 用于 upsert。

**三库兼容**：用 GORM v2 `clause.OnConflict.Assignments`，由 GORM 翻译为 PostgreSQL `ON CONFLICT … DO UPDATE`、MySQL `INSERT … ON DUPLICATE KEY UPDATE`、SQLite `INSERT … ON CONFLICT DO UPDATE`，统一在应用层做累加。

### 3.2 Why a new table

- `quota_data` 缺 `channel_id`，加列会污染既有查询；新表职责单一便于测试。
- 命中率维度（read / input / write）拆 3 列保留原始数据，命中率永远是查询时除法。

## 4. Write Path (in-memory buffer + periodic flush)

参照 `model/usedata.go:67` `SaveQuotaDataCache` 的成熟模式。

### 4.1 埋点位置

`service/text_quota.go:462` `model.RecordConsumeLog(...)` 调用之前/之后，从 `summary` 直接读取：
- `summary.CacheTokens` → `cache_read_tokens`
- `summary.PromptTokens` → `input_tokens`（已扣 cache_read）
- `summary.CacheCreationTokens` → `cache_write_tokens`

任务计费链路（`service/task_billing.go`）、违规扣费（`service/violation_fee.go`）不接入。

### 4.2 缓冲与 flush

**开关语义已确认**：后端始终采集，`console_setting.cache_hit_stats_enabled` 仅控制前端入口。埋点不做开关短路。

```go
// service/cache_hit_stats.go
func RecordCacheHitStats(userId, channelId int, modelName string,
    cacheRead, input, cacheWrite int) {
    if cacheRead == 0 && input == 0 && cacheWrite == 0 {
        return // 极端情况下没有任何 token 时不写
    }
    hour := common.GetTimestamp() - common.GetTimestamp()%3600
    model.LogCacheHitStats(userId, channelId, modelName, hour,
        cacheRead, input, cacheWrite)
}
```

```go
// model/cache_hit_stats.go
var (
    cacheHitStatsBuffer    = map[string]*CacheHitStats{}
    cacheHitStatsBufferMu  sync.Mutex
)

func LogCacheHitStats(userId, channelId int, model string, hour int64,
    cacheRead, input, cacheWrite int) {
    key := fmt.Sprintf("%d-%d-%s-%d", userId, channelId, model, hour)
    cacheHitStatsBufferMu.Lock()
    defer cacheHitStatsBufferMu.Unlock()
    if v, ok := cacheHitStatsBuffer[key]; ok {
        v.CacheReadTokens  += int64(cacheRead)
        v.InputTokens      += int64(input)
        v.CacheWriteTokens += int64(cacheWrite)
        v.RequestCount++
        return
    }
    cacheHitStatsBuffer[key] = &CacheHitStats{
        UserId: userId, ChannelId: channelId, ModelName: model,
        HourBucket: hour,
        CacheReadTokens: int64(cacheRead), InputTokens: int64(input),
        CacheWriteTokens: int64(cacheWrite), RequestCount: 1,
    }
}

func SaveCacheHitStatsCache() { /* upsert via OnConflict.Assignments(增量累加) */ }
```

### 4.3 调度

- `main.go` 启动一个 goroutine：
  ```go
  go model.UpdateCacheHitStats() // 体内 for 循环 sleep DataExportInterval 分钟后调 SaveCacheHitStatsCache
  ```
- 多节点：所有节点 flush，主键冲突走 ON CONFLICT 累加，不需要 master gating。

## 5. HTTP API

### 5.1 路由（`router/api-router.go`）

新增 group `/api/cache_hit_stats`：

| 方法 | 路径 | 鉴权 | 用途 |
|---|---|---|---|
| GET | `/me` | UserAuth | 当前用户：今日 / 历史累计 / 自定义区间 三档 summary |
| GET | `/channels` | AdminAuth | 按渠道聚合列表 |
| GET | `/models` | AdminAuth | 按模型聚合列表，支持 `channel_id=` 下钻 |
| GET | `/by_channel_model` | AdminAuth | (channel, model) 二维明细，行展开下钻使用 |

### 5.2 入参

通用：
- `start_time`、`end_time`（unix 秒，闭区间）；或者
- `range=today|7d|30d|all`（缺省 `today`，与 `start_time` 互斥）。

`/models` 额外可选：`channel_id`。

### 5.3 出参（统一）

```json
{
  "success": true,
  "data": {
    "items": [
      {
        "channel_id": 12,
        "channel_name": "Foo",
        "channel_type": 1,
        "model_name": "gpt-4o",
        "cache_read": 77700,
        "input": 413,
        "cache_write": 8600,
        "total": 86713,
        "hit_rate": 89.57,
        "request_count": 234
      }
    ],
    "summary": {
      "cache_read": 0,
      "input": 0,
      "cache_write": 0,
      "total": 0,
      "hit_rate": null,
      "request_count": 0
    }
  }
}
```

`/me` 响应特化：
```json
{
  "success": true,
  "data": {
    "today":    { "cache_read": ..., "input": ..., "cache_write": ..., "total": ..., "hit_rate": ..., "request_count": ... },
    "lifetime": { ... },
    "range":    { ... }   // 可选，仅当请求带 start/end 时返回
  }
}
```

### 5.4 命中率计算

- 在 controller 层做：`hit_rate = total > 0 ? cache_read / total * 100 : null`，三库 `NULL/0` 行为不一致，统一在应用层处理。
- 保留两位小数（`*100` 后再做 `math.Round*100/100`）。

### 5.5 渠道名/类型回填

`channels` / `by_channel_model` 接口在聚合后用 `model.CacheGetChannel(id)` 一次性补 `Name` / `Type`，缓存命中失败回退 `Name=""`、`Type=0`，不抛错。

## 6. Frontend

双前端同步：`web/default`（React 19 + Base UI）+ `web/classic`（React 18 + Semi UI）。

### 6.1 系统设置 → 侧边栏管理 → 管理员区域 新增开关

- key：`console_setting.cache_hit_stats_enabled`（默认 `false`）
- default：在 `web/default/src` 系统设置（侧边栏管理→管理员区域）相应组件内增加 toggle，沿用 `console_setting.*` 现有写入流程
- classic：`web/classic/src/components/settings/DashboardSetting.jsx` 在管理员区域加一项 toggle
- 后端验证：`controller/option.go` 现有 `console_setting.*` validation 链可以补一条（或保持默认透传，bool 字段无需校验内容）

### 6.2 侧边栏新页"缓存命中率"

**可见条件**：`role >= admin && console_setting.cache_hit_stats_enabled === true`。前端从 status / option 接口读取这两个值。

**路由**：
- default：`/console/cache-hit-stats`，挂入 `routes/_authenticated/...`
- classic：`pages/CacheHitStats/index.jsx`，在 `router/index.jsx` 注册管理员路由

**布局**：
```
┌─────────────────────────────────────────────┐
│ 时间范围: [今日|近7天|近30天|全部|自定义]      │
├─────────────────────────────────────────────┤
│ 总览卡: cache_read | input | cache_write     │
│         总计 | 命中率%                        │
├─────────────────────────────────────────────┤
│ Tabs: [按渠道] [按模型]                       │
├─────────────────────────────────────────────┤
│ 表格列: 渠道名 | 类型 | cache_read | input    │
│        cache_write | 总计 | 请求数 | 命中率   │
│        行点击 → 子表展开按模型拆分              │
└─────────────────────────────────────────────┘
```

下钻：在"按渠道" tab 下行展开时请求 `/api/cache_hit_stats/models?channel_id=X&...`；在"按模型" tab 下不再支持二次下钻。

空态：`暂无数据` 与现有 Status 页一致。

### 6.3 数据看板：今日 / 历史累计命中率卡片

- default：`web/default/src/features/dashboard/components/overview/cache-hit-rate-card.tsx` 注册到 `section-registry.tsx` 概览区
- classic：`web/classic/src/pages/Detail/index.jsx` 顶部统计卡区域追加两张卡

**数据**：`GET /api/cache_hit_stats/me`，使用 `today` / `lifetime` 两档；hover/点开 popover 展开 `cache_read / input / cache_write / 总计 / 请求数` 明细。

**可见性**：所有用户。开关只影响 6.2 的入口。

### 6.4 i18n

需在 zh 文件新增（en/fr/ru/ja/vi 由 `bun run i18n:sync` 走 CLI）：
- `启用缓存命中率统计`
- `开启后管理员可见侧边栏的"缓存命中率"入口`
- `缓存命中率`
- `缓存读取` / `输入` / `缓存创建`（与 usage-logs 命名对齐）
- `命中率` / `请求数` / `今日` / `历史累计` / `总计`
- `按渠道` / `按模型`

## 7. Migrations & Backfill

- AutoMigrate 走现有 `model.InitDB` 流程加 `cache_hit_stats` 表。
- **不回填历史 `logs.Other`**：成本高、跨库 JSON 抽取兼容差；首次启用后从写入时刻开始统计。
- UI 提示：管理员侧边栏"缓存命中率"页与用户数据看板卡片均加一条 tooltip / footnote："自启用之时起统计"。

## 8. Testing

### 8.1 Unit
- `model/cache_hit_stats_test.go`：buffer 累加、flush upsert、三库 ON CONFLICT 行为（用 SQLite in-memory 跑 happy path；MySQL/Postgres 由 CI 矩阵覆盖）。
- 命中率计算：`total = 0` 返回 nil；正常区间精确到 2 位小数。
- `RecordCacheHitStats` 在 cache_read+input+cache_write 全 0 时不入缓冲。

### 8.2 Integration
- 模拟 `RecordConsumeLog` 触发若干次（不同 user/channel/model/hour），driver flush，校验聚合结果。
- API：`/me` 三档 summary 边界（无数据 → null hit_rate）；`/channels` 区间过滤准确性；`/models?channel_id=` 下钻准确性。

### 8.3 Frontend
- 不强制 e2e；手测 default + classic：开关切换是否影响入口；"按渠道"行展开下钻是否拉到正确数据；卡片 hover 明细。

## 9. Performance

- 单条写入命中内存 map，flush 周期内同 key 永远命中累加；burst 仅产生唯一 key 数量级的 row 写入。
- 查询：`(user_id, hour_bucket)` / `(channel_id, hour_bucket)` 复合索引覆盖；30 天用户最多 720 行；管理员区间多渠道时仍是 `≤ 渠道数 × 模型数 × 区间小时数`，可控。
- 内存上限：`flush 间隔 × QPS × distinct (user, channel, model)` 数量；正常 5 万 QPS 也不到 100 MB；不需要 LRU。

## 10. Open Questions

- (无 — 全部澄清完成)
