# 模型 Uptime 监控（模型状态页面）— 设计文档

- 日期：2026-05-13
- 范围：仅 `web/classic` 前端 + 后端
- 入口：系统管理 → 运营设置 → 侧边栏管理 → 控制台区域 → "模型状态"开关；启用后侧边栏出现"模型状态"菜单项，路由 `/console/model-status`
- 关联设计：`docs/superpowers/specs/2026-05-13-channel-uptime-monitoring-design.md`（渠道 Uptime，已实现）

## 1. 背景与目标

仓库已落地"渠道 Uptime 监控"（`channel_uptime_records` 表 + `controller/channel_uptime.go` + `pages/Status/index.jsx`）。它回答的是**"渠道 X 当前是否健康"**——每次 `testAllChannels` 自动测试时只对每个 channel 用**一个测试模型**做探测。

本次新增**"模型 Uptime 监控"**：回答**"模型 M 在所有承载它的渠道上整体是否可用"**。

判定方式：完全**衍生计算**，复用 `channel_uptime_records`，按模型聚合 abilities 表中对应的所有 channel。不新增表、不新增 worker、不新增上游真实探测开销。

### 1.1 已知语义边界

`channel_uptime_records` 不记录"被测的具体模型名"——每个 channel 每轮测试用的是 `channel.TestModel`（或第一个支持模型 / 默认 `gpt-4o-mini`）。因此"模型 M 状态 = 服务 M 的所有 channel 的健康状态聚合"实际回答的是**"承载 M 的渠道们整体上是不是健康"**，并不等价于**"M 这个具体模型在这些渠道上能不能跑"**。

这是用户在方案选择阶段已经明确接受的折衷——通过零探测成本换取近似可用性视图。

## 2. 设计决策（已确认）

| 决策点 | 选择 |
|---|---|
| 判定方式 | 复用 `channel_uptime_records`，按模型聚合（无新表、无 worker） |
| 前端范围 | 仅 `web/classic`，不动 `web/default` |
| 可见性 | 管理员视图 vs 普通用户视图，两种结构 |
| 分组维度 | 管理员跨组；普通用户按 `user.Group` 过滤 abilities |
| 路由 | 单端点 `GET /api/model-uptime/status`，role 分支 |
| 鉴权 | `middleware.UserAuth()` |
| 时间窗 | 24h、75 桶 |
| 状态语义 | `normal` / `degraded` / `error` / `unknown` |
| 失败排序 | 失败模型沉顶（按 status 优先级 → model 字母序） |
| 延迟显示 | 模型层不显示；管理员展开 channel 详情时显示 per-channel |
| 触发依赖 | 无独立开关；依赖 `AutoTestChannelEnabled` |
| 缓存 | 无（YAGNI，有压力再加） |

## 3. 数据模型与查询逻辑

**不新增表**。所有数据来自现有：

| 表 | 用途 | 关键字段 |
|---|---|---|
| `abilities` | 找出每个模型由哪些 channel 服务 | `model`, `channel_id`, `enabled`, `group` |
| `channel_uptime_records` | 已有的健康历史 | `channel_id`, `status`, `status_code`, `response_time_ms`, `error_message`, `created_time` |
| `channels` | 名称/类型映射（管理员视图用） | `id`, `name`, `type`, `status` |

### 3.1 过滤规则

```go
type modelUptimeFilter struct {
    Groups []string // nil 或空 = 不按组过滤（管理员视图）
}
```

- **管理员**：`Groups = nil`，扫所有 `enabled=true` 的 abilities
- **普通用户**：`Groups = strings.Split(user.Group, ",")`（剔除空字符串），SQL `WHERE enabled=? AND <group> IN (?)`

普通用户 `user.Group=""`（罕见、账户损坏）→ 返回 `models: []`，不报错。

`user.Group` 通过 `model.GetUserGroup(userId, true)` 获取（已有函数）。

### 3.2 查询策略（3 次 SQL，与模型数无关）

```go
// 1) 拉出 (model, channel_id) 映射；group 列用 commonGroupCol 转义
DB.Table("abilities").
    Select("model, channel_id").
    Where("enabled = ?", commonTrueVal).
    Where(...optional commonGroupCol+" IN ?", groups).
    Find(&rows)

// Go 侧 dedupe：同一 (model, channel_id) 在不同 group 下可能多次出现
modelToChannels := map[string]map[int]struct{}{}

// 2) 一次拉所有相关 channel 过去 24h 的记录
DB.Select("channel_id, status, status_code, response_time_ms, error_message, created_time").
    Where("channel_id IN ? AND created_time >= ?", allChannelIds, dayAgo).
    Find(&recent)

// 3) 每 channel "最新一条"
//    方案：对 recent 在 Go 侧按 channel_id 求最大 created_time（O(n)）
//    理由：避免数据库特定的窗口函数 ROW_NUMBER()，SQLite / 老 MySQL 不一致
```

**为何 Go 侧 reduce 而不是 SQL 窗口函数**：PostgreSQL 支持 `ROW_NUMBER() OVER`，但 SQLite/老 MySQL 不一致；在 Go 侧 reduce 几万条记录非常便宜（O(n)），代码简单且三库可移植（CLAUDE.md Rule 2）。

### 3.3 聚合规则

**当前状态**（每个模型）——遍历它的 channel 集合，取每个 channel 的最新记录：

| 各 channel 最新记录情况 | 模型状态 |
|---|---|
| 至少一个 success，没有 failure | `normal` |
| success 与 failure 共存 | `degraded` |
| 只有 failure，没有 success | `error` |
| 全无记录 | `unknown` |

**24h 可用率**（每个模型）：

```
uptime_24h = SUM(status=success in 24h across all serving channels)
           / COUNT(records in 24h across all serving channels) * 100
```

无记录 → `null`（前端显示 "-"）。

**75 桶历史**（24h，每桶约 19.2 分钟）：

- 桶起止时间生成与 `model/channel_uptime.go` 完全一致（复用常量 `channelUptimeBucketCount=75`）
- 每个桶遍历**该模型所有 channel 在该桶内的记录**：
  - 至少一条 success → `1`
  - 全部 failure → `0`
  - 无记录 → `-1`
- 字段：`{status, ts_start, ts_end, sample_size}`

### 3.4 管理员视图额外字段：per-channel 当前快照

对每个模型，附带 `channels: [...]`，每个元素是它的某个支持 channel 的**当前快照**（只一条最新记录，不带历史，控制 payload 大小）：

```go
type ModelChannelSnapshot struct {
    Id           int    `json:"id"`
    Name         string `json:"name"`
    Type         int    `json:"type"`
    Status       string `json:"status"`        // normal / error / unknown
    StatusCode   int    `json:"status_code"`
    LatencyMs    int    `json:"latency_ms"`
    LastCheck    int64  `json:"last_check"`
    ErrorMessage string `json:"error,omitempty"`
}
```

- channel `name` / `type` 从 channels 表一次性预热的 map 取（一次 `model.GetAllChannels` 拿到，不需要逐个查询）
- 跳过 `ChannelStatusManuallyDisabled` 的 channel（与 testAllChannels 行为一致）
- 普通用户响应里**绝对不包含**这个字段

### 3.5 排序

- 管理员/普通用户均按 `status` → `model` 字母序排：`error` → `degraded` → `unknown` → `normal`
- 失败的模型沉到顶部，便于一眼看到问题
- 同状态内按 `model` 字段升序

### 3.6 跨库注意点（CLAUDE.md Rule 2）

- `enabled = ?` 用 `commonTrueVal` / `commonFalseVal`（`model/main.go` 已有）
- 列名 `group` 用 `commonGroupCol`（`model/main.go` 已有）转义
- 时间字段用 `created_time` Unix 秒比较，无 DB 特定函数
- 不使用 `GROUP_CONCAT` / 窗口函数 / JSONB 操作符

### 3.7 复杂度

- 时间：O(abilities 行数 + 24h 记录数)，典型几万条以下，亚秒级
- 空间：3 个查询结果常驻内存计算；最大约 30k uptime 记录 × ~64 B ≈ 2 MB
- 无后台任务、无锁、无 Redis 依赖

## 4. 后端 API

### 4.1 路由

在 `router/api-router.go` 现有 `channelUptimeRoute` 同级新增：

```go
modelUptimeRoute := apiRouter.Group("/model-uptime")
modelUptimeRoute.Use(middleware.UserAuth())
{
    modelUptimeRoute.GET("/status", controller.GetModelUptimeStatus)
}
```

未登录 → 401（middleware 自动）。

### 4.2 请求

`GET /api/model-uptime/status` — 无查询参数。

服务端依据 `c.GetInt("role")` 与 `c.GetInt("id")` → `model.GetUserGroup(userId, true)` 自动决定视图与过滤。

### 4.3 管理员响应（`role >= common.RoleAdminUser`）

```jsonc
{
  "success": true,
  "message": "",
  "data": {
    "view": "admin",
    "interval_minutes": 5,
    "updated_at": 1747107948,
    "models": [
      {
        "model": "claude-3-opus-20240229",
        "status": "error",
        "uptime_24h": 12.5,
        "last_check": 1747107948,
        "channel_count": 2,
        "healthy_count": 0,
        "history": [
          { "status": 0, "ts_start": 1747021548, "ts_end": 1747022700, "sample_size": 4 },
          { "status": -1, "ts_start": 1747022700, "ts_end": 1747023852, "sample_size": 0 }
        ],
        "channels": [
          {
            "id": 12,
            "name": "Anthropic-Main",
            "type": 14,
            "status": "error",
            "status_code": 502,
            "latency_ms": 8421,
            "last_check": 1747107948,
            "error": "upstream error: rate limit exceeded"
          }
        ]
      }
    ]
  }
}
```

字段说明：

- `interval_minutes` — 当前 `AutoTestChannelMinutes` 设置（前端用作刷新频率参考）
- `model` — `abilities.model` 原值
- `status` — `normal | degraded | error | unknown`
- `uptime_24h` — `float64` 或 `null`
- `channel_count` — 该模型在过滤后的 abilities 集合中的去重 channel 数
- `healthy_count` — 其中 latest 为 `success` 的 channel 数
- `last_check` — 该模型所有 channel 最新记录里最大的 `created_time`；全无记录则为 `0`
- `history` — 75 桶
- `channels` — per-channel 快照数组，按 `id` 升序

### 4.4 普通用户响应（脱敏）

```jsonc
{
  "success": true,
  "message": "",
  "data": {
    "view": "public",
    "interval_minutes": 5,
    "updated_at": 1747107948,
    "models": [
      {
        "model": "claude-3-opus-20240229",
        "status": "normal",
        "uptime_24h": 99.2,
        "history": [
          { "status": 1, "ts_start": 1747021548, "ts_end": 1747022700, "sample_size": 6 }
        ]
      }
    ]
  }
}
```

**严格不包含**：`channel_count`、`healthy_count`、`channels[]`、`last_check`、任何 channel id/name/type/error/latency。

### 4.5 视图脱敏的工程约束

Controller 层用**两个独立的响应结构体**，不复用 admin 结构再删字段——避免后续新增字段时意外把内部信息泄露到 public 响应：

```go
type modelUptimeAdminEntry struct { ... }   // 完整
type modelUptimePublicEntry struct { ... }  // 脱敏
```

Controller 按 role 分支后调用各自的 builder。

### 4.6 错误与边界

| 场景 | 行为 |
|---|---|
| 用户未登录 | 401（middleware.UserAuth 自动） |
| 用户 `user.Group=""` | 普通用户返回 `models: []`，HTTP 200 |
| 系统无任何 enabled ability | 两个视图都返回 `models: []`，HTTP 200 |
| `channel_uptime_records` 完全空 | 所有模型 `status="unknown"`、`uptime_24h=null`、history 全 `-1`，HTTP 200 |
| DB 查询失败 | `common.ApiError(c, err)`（500） |

### 4.7 与 `/api/channel-uptime/status` 的并存

两个端点完全独立：

- `/api/channel-uptime/status` — 渠道维度（已存在），不修改
- `/api/model-uptime/status` — 模型维度（本次新增）

前端两个页面（"服务状态" / "模型状态"）各自独立调用。

### 4.8 性能与速率

无 cache、每次请求都查库。基于 §3.7 估算，单次响应 200–500ms 内。如果将来发现压力问题，可在 controller 上加 30s TTL 内存缓存——**当前版本不做**（YAGNI）。

页面侧每 30s 自动刷新（与现有 Status 页一致）。

## 5. 前端（仅 `web/classic`）

### 5.1 文件改动一览

| 文件 | 改动 |
|---|---|
| `web/classic/src/pages/ModelStatus/index.jsx` | **新建** 页面组件 |
| `web/classic/src/App.jsx` | 新增 `/console/model-status` 路由 |
| `web/classic/src/hooks/common/useSidebar.js` | `consoleItems` 中加 `modelStatus` 条目 |
| `web/classic/src/components/layout/SiderBar.jsx` | 渲染 `modelStatus` 菜单项（位于"服务状态"下方） |
| `web/classic/src/pages/Setting/Operation/SettingsSidebarModulesAdmin.jsx` | `console.modules` 数组追加 `modelStatus` 开关 + 默认值/归一化 |
| `web/classic/src/i18n/locales/{zh,zh-CN,zh-TW,en,fr,ru,ja,vi}.json` | 补 i18n key |
| `web/classic/src/helpers/render.jsx` | 若需新图标，加一个引用（否则不动） |

### 5.2 配置开关

在 `SettingsSidebarModulesAdmin.jsx`：

- 默认结构 `console` 区追加 `modelStatus: true`（共 4 处：初始 state、`resetSidebarModules` 默认、`useEffect` 归一化补全、空配置兜底）
- `sectionConfigs.console.modules` 数组追加：

  ```js
  { key: 'modelStatus', title: t('模型状态'), description: t('模型可用性监控') }
  ```

开关存储于 `Option` 表的 `SidebarModulesAdmin` JSON 字符串中（已存在机制），无需后端额外字段。

### 5.3 路由 & 侧边栏

**`App.jsx`**：在现有 `/console/status` 路由附近添加

```jsx
<Route path='/console/model-status' element={
  <PrivateRoute><ModelStatus /></PrivateRoute>
} />
```

并 lazy import：`const ModelStatus = lazy(() => import('./pages/ModelStatus'));`

**`useSidebar.js`**：

- 在 `routerMap` 加 `modelStatus: '/console/model-status'`
- 在 `consoleItems` 中"服务状态"项之后追加 `modelStatus` 项，遵循现有 `enabled`/`visible` 联动逻辑

**`SiderBar.jsx`**：照"服务状态"项 fork 一份，`itemKey`/`text`/`to`/`icon` 改为 modelStatus；图标用 `lucide-react` 的 `Boxes`（"成组的盒子"，区别于"服务状态"用的 `Activity`）。

### 5.4 页面行为 — `pages/ModelStatus/index.jsx`

**布局**：竖向**列表**（不是 grid），原因是模型数可能远多于渠道数（真实部署轻松 50–200 个模型）。

**顶部条**：

- 左：标题"模型状态" + 副标题
- 中：**搜索框**（按 `model` 字段子串匹配，前端过滤，不打额外 API）
- 中：**状态筛选**——4 个 chip：`正常 / 降级 / 故障 / 未知`（多选）
- 右：`上次更新于 HH:mm:ss` + "立即刷新"按钮

**统计卡片**（同 Status 页风格，但语义改为模型维度）：

- 正常 / 降级 / 故障 / 未知模型数（按 `models[]` 计）

**列表项**（单行紧凑卡）：

- 模型名 + 状态徽章 + 24h 可用率
- 75 桶 history strip（绿/红/灰三色）
- 鼠标 hover 单个桶 → `Popover` 显示该桶起止时间 + sample_size + 状态文案
- 列表项底色按 status 微调（error 浅红、degraded 浅黄、normal 透明、unknown 浅灰）

**管理员"展开"**：点击行 → 内嵌一个轻量 Semi UI `Table`：

| Channel | 类型 | 状态 | 状态码 | 延迟 | 最近检查 | 错误 |
|---|---|---|---|---|---|---|
| Anthropic-Main | claude | error | 502 | 8.4s | 12:45:33 | upstream error: rate limit exceeded |

`channel name` / 错误超长 → `Popover` 显示完整文本。普通用户视图**不渲染**这块代码——通过 `view === 'admin'` 守卫，不是"渲染后用 CSS 隐藏"。

**自动刷新**：`setInterval(fetch, 30 * 1000)`。组件卸载 `clearInterval`。

**加载/空态**：

- 首次加载 → 骨架屏（Card + `Skeleton`）
- 接口返回 `models: []` → `<Empty>` "暂无模型"
- 网络错误 → `<Empty>` "加载失败"（不弹 toast，与现有 Status 页一致）

### 5.5 i18n key（中文源串列表）

新增 key（同步脚本 `bun run i18n:sync` 会按现有字段补到其他语言）：

```
模型状态
模型可用性监控
查看模型在所有承载渠道上的近 24 小时可用性
搜索模型
正常模型
降级模型
故障模型
未知模型
24小时可用率
最近检查
状态码
错误
展开渠道
收起渠道
暂无模型
未知类型
```

复用已有 key：`立即刷新` / `更新于` / `延迟` / `毫秒` / `秒` / `服务状态` / `正常` / `异常` / `故障` 等无需重复添加。

### 5.6 不动的部分

- `web/default` 任何文件
- 现有 `/console/status` 页面（"服务状态"）代码与样式
- `channel_uptime_records` 表与 testAllChannels 写入逻辑
- 后端 i18n（`i18n/locales/*.json`，因为接口不返回需要本地化的文案）

### 5.7 不改的接口

- 不在 `/api/option` 加新 key（开关复用 `SidebarModulesAdmin`）
- 不在 `/api/status` 暴露新字段

### 5.8 直达 URL 访问

`/console/model-status` 与 `/console/status` 行为对齐：

- `modelStatus` 开关关闭仅影响侧边栏菜单可见性
- 直达 URL 仍可访问（后端不做 toggle 拒绝），与现有 sidebar modules 一致

## 6. 测试与验收

### 6.1 后端单元测试（`model/model_uptime_test.go`）

仿 `model/channel_uptime_test.go` 在 SQLite 内存库下运行。覆盖以下 9 个 case：

| # | 场景 | 期望 |
|---|---|---|
| 1 | abilities 表空 | 查询返回 `models: []`，无 panic |
| 2 | 单模型单 channel，最近 1 条 success | `status=normal`，`uptime_24h=100`，history 至少 1 个 `1` |
| 3 | 单模型双 channel，一个 success 一个 failure | `status=degraded`，channel_count=2，admin healthy_count=1 |
| 4 | 单模型双 channel 全 failure | `status=error`，admin per-channel 两个 entry |
| 5 | 单模型 channel 集合中所有 channel 都从未有记录 | `status=unknown`，`uptime_24h=null`，history 75 个 `-1` |
| 6 | 跨组：abilities 同模型在 default/vip 由不同 channel 提供，传 `Groups=["vip"]` | 只取 vip 那条 channel |
| 7 | 用户多组：`Groups=["a","b"]` | abilities WHERE group IN (a,b) 行为正确（dedupe channel 不重复计数） |
| 8 | 24h 边界：channel 一周前有 success，24h 内只有 failure | `uptime_24h` 仅基于 24h 内（应为 0），history 大部分 `-1` |
| 9 | abilities `enabled=false` 不参与 | 该 channel 不出现在 channel_count 中 |

**禁用渠道**：测试 `ChannelStatusManuallyDisabled` 的 channel 不出现在 admin 视图的 `channels[]` 里（一条专门 case 验证）。

### 6.2 跨库手测（Rule 2）

| DB | 检查点 |
|---|---|
| SQLite | 默认，单测全过即可 |
| MySQL 5.7.8 | `enabled=?` 用 `commonTrueVal=1` 工作；`group` 字段反引号包裹 |
| PostgreSQL | `enabled=?` 用 `commonTrueVal=true` 工作；`"group"` 双引号包裹 |

手测步骤：建库 → 跑迁移 → 插一条 ability + 一条 uptime 记录 → 请求接口断言 200 + 正确字段。三库都过才能合并。

### 6.3 集成手测剧本

1. **管理员场景**：
   - 启用自动测试（`AutoTestChannelEnabled=on`，5 分钟间隔），等待写入 2 轮以上
   - 以 admin 登录访问 `/console/model-status`
   - 验证：见所有 enabled abilities 涉及的模型；故障模型沉顶；展开某行可见 per-channel 表
   - 关掉某个 channel 的上游 → 等下一轮探测 → 刷新页面 → 涉及该 channel 的模型 status 落入 `error`/`degraded`

2. **普通用户脱敏**：
   - 普通用户登录访问同 URL
   - 浏览器 Network 面板检查响应**绝对不包含**：`channels[]`、`channel_count`、`healthy_count`、`last_check`、任何 channel name/id/type/error/latency
   - 这一点必须**人肉看 raw JSON**确认（自动化难以可靠覆盖）

3. **组过滤**：
   - 创建两个 group `gA`/`gB`，一个 channel 仅在 `gA` 提供 `gpt-4o-mini`
   - 普通用户 `user.Group=gB` 登录 → 响应里不出现 `gpt-4o-mini`
   - 切换到 `user.Group=gA` → 出现

4. **开关**：
   - 运营设置关闭 `console.modelStatus` 开关 → 普通用户菜单消失
   - 直达 `/console/model-status` 仍可访问（与 `console.status` 行为一致）

5. **空态**：
   - 全新部署，未启用自动测试 → 接口返回所有模型 `status=unknown`，页面渲染列表（非空），每行 history 75 个灰桶

### 6.4 回归检查点

- `testAllChannels` / channel uptime 写入逻辑**未被触碰**
- `/api/channel-uptime/status` 响应字段与字节顺序未变
- `SidebarModulesAdmin` 现有键（`status`、`detail`、`token` 等）行为不变
- 现有 `/console/status` 页面渲染未变（仅在同一 i18n 文件新增 key）

### 6.5 验收门槛

合并前必须满足：

- 单测 9 个 case 全过
- `go vet ./...` 与 `gofmt -d` 干净
- 三库手测通过（SQLite/MySQL/PostgreSQL 至少各跑过一次集成剧本 6.3.1）
- 脱敏验证 6.3.2 人肉确认
- `bun run build` 在 `web/classic/` 下成功（无 TS / lint 报错）

## 7. 非目标（明确不做）

- **`web/default` 实现** —— 单独立项
- **真实模型探测**（每个 model 单独 testChannel）—— 已在方案选择阶段否决
- **告警 / 通知** —— 后续增量
- **历史趋势图**（折线 / 热力图）—— 后续增量
- **保留期可配置** —— 复用 channel_uptime 的硬编码 7 天，不引入新设置
- **per-model 延迟统计** —— §1.1 已说明语义边界，不在此版本提供
- **响应缓存 / Redis** —— YAGNI，先上裸查询，有压力再加

## 8. 实施顺序建议

供后续 plan 参考：

1. 后端：`model/model_uptime.go`（查询函数）+ 单测
2. 后端：`controller/model_uptime.go` + 路由注册
3. 三库本地手测后端
4. 前端：路由 + 侧边栏菜单 + 开关 UI
5. 前端：`ModelStatus` 页面 + i18n
6. 集成手测 §6.3
7. 提交

## 9. 文件清单

**新建**：

- `model/model_uptime.go`
- `model/model_uptime_test.go`
- `controller/model_uptime.go`
- `web/classic/src/pages/ModelStatus/index.jsx`
- `docs/superpowers/specs/2026-05-13-model-uptime-monitoring-design.md`（本文档）

**修改**：

- `router/api-router.go` — 注册 `GET /api/model-uptime/status`
- `web/classic/src/App.jsx` — 注册 `/console/model-status` 路由
- `web/classic/src/hooks/common/useSidebar.js` — 加 `modelStatus` 菜单项
- `web/classic/src/components/layout/SiderBar.jsx` — 加菜单项渲染
- `web/classic/src/pages/Setting/Operation/SettingsSidebarModulesAdmin.jsx` — `console` 区追加 `modelStatus` 开关
- `web/classic/src/i18n/locales/*.json` — 补 i18n key

**保护信息**：本设计不修改任何 QuantumNous / new-api 相关品牌、版权、模块路径或元数据（CLAUDE.md Rule 5）。
