# 使用统计页 — 错误率指标 设计文档

- 日期：2026-06-13
- 分支：`hidden-price`
- 状态：已与用户确认设计方向，待 spec 审查

## 1. 背景与目标

new-api 是一个聚合 40+ 上游 AI 供应商的 API 网关。每次请求都会在 `logs` 表落一条记录：成功消费记为 `Type=LogTypeConsume(2)`，失败记为 `Type=LogTypeError(5)`。

目标：新增一个**「使用统计」页面**，向用户与管理员展示 API 请求的**错误率**指标，支持时间窗口选择与（管理员）按渠道/标签筛选。

「使用统计」是一个可扩展的容器命名，**本次只实现错误率一个指标**，其它使用指标（请求量、Token、RPM 等）留待后续。

## 2. 需求确认（已与用户敲定）

| 维度 | 结论 |
|---|---|
| 错误率口径 | `error_rate = 错误数 ÷ (错误数 + 成功数)`，即 `count(type=5) ÷ (count(type=5) + count(type=2))` |
| 展示形态 | 一个「总错误率」数字（含 error/success/total）+ 错误率随时间的**趋势折线** |
| 时间窗口 | `5m` / `30m` / `1h` / `2h` / `1d`（最近 N），默认 `1h` |
| 筛选（管理员） | 按指定**渠道 channel_id**、指定**标签 tag** 筛选 |
| 权限视角 | 普通用户看**自己账号**错误率（不可按渠道/标签筛选）；管理员看**全局**并可按渠道/标签筛选 |
| 页面位置 | web/classic 新页面，路由 `/console/usage-stats`，侧边栏挂在「数据看板」附近；用户 + 管理员都可入 |
| 开关 | 系统设置加页面开关（仿 `CacheHitStatsEnabled`） |
| 前端范围 | **仅改 web/classic，不碰 web/default** |

## 3. 非目标（YAGNI）

- 不新建预聚合表（数据策略选「实时查 logs」，见 §5）。
- 不做错误率以外的其它使用指标。
- 不做告警/阈值通知。
- 不做按用户、按模型维度的分组展示（仅全局总错误率 + 趋势；管理员可用 channel/tag 作为过滤条件，而非分组）。

## 4. 架构与文件清单

分层：Router → Controller → Service/Model。数据实时查 `logs` 表。

### 后端（Go）

| 文件 | 改动 | 说明 |
|---|---|---|
| `model/usage_stats.go` | 🆕 新建 | 错误率聚合查询：总计 + 趋势分桶，含筛选与三库兼容 |
| `controller/usage_stats.go` | 🆕 新建 | 两个 handler：`GetMyErrorRate`（用户）/`GetErrorRate`（管理员） |
| `router/api-router.go` | ✏️ 修改 | 注册 `/api/usage_stats/error_rate` 与 `/me` 路由 |
| `setting/console_setting/config.go` | ✏️ 修改 | 加 `UsageStatsEnabled bool json:"usage_stats_enabled"` |
| `controller/misc.go` | ✏️ 修改 | 在状态接口下发 `usage_stats_enabled` |

### 前端（web/classic）

| 文件 | 改动 | 说明 |
|---|---|---|
| `web/classic/src/pages/UsageStats/index.jsx` | 🆕 新建 | 页面主体，参考 `pages/CacheHitStats/index.jsx` |
| `web/classic/src/App.jsx` | ✏️ 修改 | `lazy` 引入 + 路由 `/console/usage-stats` |
| `web/classic/src/components/layout/SiderBar.jsx` | ✏️ 修改 | `workspaceItems` 加「使用统计」项 |
| `web/classic/src/pages/Setting/Operation/SettingsSidebarModulesAdmin.jsx` | ✏️ 修改 | 侧边栏模块管理加 `usageStats` 项 |
| `web/classic/src/i18n/locales/*.json` | ✏️ 修改 | 中英等文案 key |

## 5. 数据策略：实时查 `logs`

直接查 `logs` 表实时聚合，复用 `model/log.go:SumUsedQuota`（约 line 435）已验证的三库兼容筛选写法（`created_at` 范围、`channel_id`、`logGroupCol`、`model_name LIKE`）。

理由：时间窗口最长 1 天，`logs.created_at` 有索引（`idx_created_at_id` / `idx_created_at_type`），实时查足够快；无需新表 / 迁移 / 写日志埋点，数据实时准确。预聚合表对该场景属过度设计。

## 6. 后端 API 设计

### 路由（`router/api-router.go`，与 `cacheHitRoute` 同区注册）

```
GET /api/usage_stats/error_rate/me   (middleware.UserAuth())   — 普通用户看自己
GET /api/usage_stats/error_rate      (middleware.AdminAuth())  — 管理员看全局
```

### 请求参数

| 参数 | 类型 | 说明 |
|---|---|---|
| `window` | string | `5m`/`30m`/`1h`/`2h`/`1d`，非法值回退默认 `1h` |
| `channel_id` | int | 可选；**仅管理员接口**生效 |
| `tag` | string | 可选；**仅管理员接口**生效 |

- `/me`：强制 `WHERE user_id = c.GetInt("id")`，**忽略** `channel_id`/`tag`（渠道信息不对普通用户暴露）。
- `tag` 处理：调 `model.GetChannelsByTag(tag, ...)`（model/channel.go:337）取出该标签下 channel id 列表，转为 `WHERE channel_id IN (...)`；列表为空则结果为空。
- `channel_id` 与 `tag` 同时传：两者都作为条件叠加（AND）。

### 返回 DTO（JSON 经 `common.*` 序列化，遵守项目 Rule 1）

```json
{
  "success": true,
  "message": "",
  "data": {
    "window": "1h",
    "error_count": 12,
    "success_count": 388,
    "total": 400,
    "error_rate": 3.0,
    "trend": [
      { "time": 1718200000, "error_count": 1, "success_count": 40, "error_rate": 2.44 },
      { "time": 1718200300, "error_count": 0, "success_count": 0,  "error_rate": 0 }
    ]
  }
}
```

- `time`：桶起始 unix 秒。
- 失败时遵循现有风格返回 `{"success": false, "message": "..."}`。

## 7. 后端数据查询设计（核心）

### 7.1 窗口 → (时长, 桶大小, 桶数) 映射

| window | 时长(s) | 桶大小(s) | 桶数 |
|---|---|---|---|
| `5m` | 300 | 30 | 10 |
| `30m` | 1800 | 120 | 15 |
| `1h` | 3600 | 300 | 12 |
| `2h` | 7200 | 600 | 12 |
| `1d` | 86400 | 3600 | 24 |

时间范围按桶网格对齐，使趋势桶数恒定 = 桶数：

- `end = now`
- `lastBucket = (now / bucketSize) * bucketSize`（当前桶起始，整除）
- `start = lastBucket - (桶数 - 1) * bucketSize`（即趋势第一个桶的起始）

趋势恰好覆盖 `firstBucket(=start), firstBucket+bucketSize, …, lastBucket` 共「桶数」个桶。

### 7.2 计数口径（CASE WHEN，三库通用）

```sql
SUM(CASE WHEN type = 5 THEN 1 ELSE 0 END) AS error_count
SUM(CASE WHEN type = 2 THEN 1 ELSE 0 END) AS success_count
```

`error_rate` 在 Go 层计算：`round(error/(error+success)*10000)/100`（保留 2 位小数，%）。**`total = error+success = 0` 时 `error_rate = 0`**（区别于 cache_hit 的返回 `nil`，便于前端连续画线）。

### 7.3 趋势分桶（三库兼容，关键难点）

按**绝对时间网格**对齐分桶，桶表达式只含常量 `bucketSize`，不含 `start`，避免 GROUP BY 重复参数：

- 桶键：`bucket = created_at / bucketSize`（整除）
- 桶起始时间：`bucket * bucketSize`

整数除法三库行为不同，按标志分支构造表达式：

```go
// SQLite / PostgreSQL: 整数 / 整数 即整除
// MySQL: / 返回小数，需 FLOOR
var bucketExpr string
if common.UsingMySQL {
    bucketExpr = "FLOOR(created_at / ?)"
} else { // UsingSQLite || UsingPostgreSQL
    bucketExpr = "(created_at / ?)"
}
```

查询（GORM）：`Select(bucketExpr + " AS bucket, SUM(CASE...) AS error_count, SUM(CASE...) AS success_count", bucketSize)`，加全部 WHERE 过滤条件后 `.Group("bucket")`。

**WHERE 子句组成（趋势与总计共用同一套过滤，保证一致）**：
- `created_at >= start AND created_at <= end`（§7.1 的对齐范围）
- `type IN (2, 5)`（只取成功消费与错误两类，并可命中 `idx_created_at_type` 索引）
- 管理员：可选 `channel_id = ?` 与/或 `channel_id IN (tag 对应 ids)`
- `/me`：`user_id = ?`

> 兼容性注记：GROUP BY 别名在 SQLite/MySQL/PostgreSQL 均被接受；实现时仍以三库（至少 SQLite + 一种）实际跑通为准。若某库 GROUP BY 别名报错，则改为 `.Group(bucketExpr_with_bucketSize_inlined)`（bucketSize 是后端可信常量，可安全内联）。

### 7.4 空桶补零

DB 只返回有数据的桶。Go 层从 `start`（=firstBucket）起逐 `+bucketSize` 生成「桶数」个桶的完整时间轴，把查询结果按 `bucket*bucketSize` 映射进去，缺失桶填 `error_count=0, success_count=0, error_rate=0`，按时间升序返回。趋势长度恒等于该窗口的桶数。

### 7.5 总计

总计 = 对 trend 各桶求和（`error_count`/`success_count` 累加，`total = error+success`，再算 `error_rate`）。因趋势与总计共用同一套 WHERE（§7.3），对 trend 求和与「单独一条不分桶聚合查询」结果一致，故优先求和，省一次 DB 往返。

### 7.6 函数签名（建议）

```go
// model/usage_stats.go
type ErrorRateBucket struct {
    Time         int64   `json:"time"`
    ErrorCount   int64   `json:"error_count"`
    SuccessCount int64   `json:"success_count"`
    ErrorRate    float64 `json:"error_rate"`
}

type ErrorRateResult struct {
    Window       string            `json:"window"`
    ErrorCount   int64             `json:"error_count"`
    SuccessCount int64             `json:"success_count"`
    Total        int64             `json:"total"`
    ErrorRate    float64           `json:"error_rate"`
    Trend        []ErrorRateBucket `json:"trend"`
}

// userId == 0 表示全局（管理员）；>0 表示限定该用户
// channelIDs == nil/空 表示不按渠道过滤
func QueryErrorRate(userId int, channelIDs []int, start, end, bucketSize int64) (ErrorRateResult, error)
```

Controller 负责：解析 `window`、（管理员）解析 `channel_id`/`tag`→`channelIDs`、调用 `QueryErrorRate`、组织 JSON。

## 8. 前端设计（web/classic，参考 `pages/CacheHitStats/index.jsx`）

- 布局：
  - 顶部：时间窗口 Button 组（`5分钟`/`30分钟`/`1小时`/`2小时`/`1天`）+ 「立即刷新」按钮。
  - 主体：**总错误率大数字卡片**（显示 error_rate%、error/success/total）+ **VChart 折线图**（X=桶时间，Y=错误率%，range 0~100）。
- 权限分支：用现有用户 context 的 `isAdmin` 判断：
  - 管理员：调 `GET /api/usage_stats/error_rate`，渲染**渠道下拉 + 标签下拉**筛选区。
  - 普通用户：调 `GET /api/usage_stats/error_rate/me`，**不渲染**筛选区。
- 渠道/标签下拉数据源：复用现有渠道列表接口（实现时确认现有 `/api/channel/...` 或下拉数据获取方式）。
- 库：`@visactor/react-vchart` + `@douyinfe/semi-ui`，主题 `initVChartSemiTheme`，与 `CacheHitStats` 一致。
- API 封装：复用 `helpers` 的 `API`、`showError`。

## 9. 开关 / 侧边栏 / 下发机制

- **后端开关**：`setting/console_setting/config.go` 加 `UsageStatsEnabled bool json:"usage_stats_enabled"`（仿 `CacheHitStatsEnabled`）。
- **下发**：`controller/misc.go`（约 line 101，`cache_hit_stats_enabled` 同处）加 `"usage_stats_enabled": cs.UsageStatsEnabled`。
- **侧边栏**：`SiderBar.jsx` 的 `workspaceItems` 加项：
  ```
  { text: t('使用统计'), itemKey: 'usageStats', to: '/console/usage-stats' }
  ```
  显隐沿用现有 `isModuleVisible('console', item.itemKey)` 机制。
- **模块管理**：`SettingsSidebarModulesAdmin.jsx` 加 `{ key: 'usageStats', title: t('使用统计'), description: t('API 请求错误率统计') }`（参考 `cacheHitStats` 项，line 320）。

## 10. i18n

- web/classic：在 `src/i18n/locales/zh.json`、`en.json` 等补充新增 key（页面标题、窗口标签、卡片文案、筛选标签、模块管理标题/描述）。以中文为源，至少补齐 zh/en。
- 后端无新增面向用户的文案（错误信息复用现有风格）。

## 11. 边界与错误处理

- `window` 非法/缺省 → 回退 `1h`。
- `/me` 接口忽略 `channel_id`/`tag`。
- `tag` 查无对应 channel → `channelIDs` 为空 → 结果各计数为 0、趋势全 0。
- `total = 0` → `error_rate = 0`；趋势空桶 → 填 0。
- DB 查询出错 → `{"success": false, "message": ...}`，并 `common.SysError` 记录。

## 12. 测试计划

`model/usage_stats_test.go`（SQLite 内存库，参考现有 model 测试）：

- 口径正确性：构造 N 条 `type=5` + M 条 `type=2`，断言 `error_rate = N/(N+M)`。
- 时间窗口：窗口外的日志不计入。
- `channel_id` 过滤、`tag`→channelIDs 过滤（含空列表）。
- 用户维度：`userId>0` 只统计该用户。
- 分桶：日志落在不同桶，断言每桶计数与桶时间正确。
- 空桶补零：中间无数据的桶返回 0，趋势长度 = bucketCount。
- 边界：`total=0` 时 `error_rate=0`。

后端跑通三库兼容（至少 SQLite + 验证 MySQL 分桶表达式 `FLOOR`）。前端手动验证两种角色视图与窗口切换、开关显隐。

## 13. 现有可复用参考点

- `model/log.go:435 SumUsedQuota` — 三库兼容筛选聚合模板。
- `controller/cache_hit_stats.go` — `parseTimeRange` / `hitRate` / `{items, summary}` 返回风格。
- `model/channel.go:337 GetChannelsByTag` — tag → channels。
- `setting/console_setting/config.go:14 CacheHitStatsEnabled` + `controller/misc.go:101` — 开关 + 下发模板。
- `web/classic/src/pages/CacheHitStats/index.jsx` — 前端页面模板（VChart + Semi + 窗口按钮组）。
- `web/classic/src/App.jsx:357` + `SiderBar.jsx:79` + `SettingsSidebarModulesAdmin.jsx:320` — 路由 / 侧边栏 / 模块管理接入点。
