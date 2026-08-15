# 代理管理设计文档

## 1. 概述

### 1.1 需求背景

当前渠道的网络代理配置以自由文本 URL 字符串保存在 `channels.settings.proxy` 字段中。这种方式存在以下问题：

- 相同代理需要在每个渠道里重复填写，无法集中管理
- 代理密码/账号变更时需要逐个渠道修改
- 无法直观了解某个代理正在被哪些渠道使用
- 无法主动检测代理的连通性

### 1.2 设计目标

- 提供独立的"代理管理"模块，代理作为一等实体统一维护
- 渠道通过 `proxy_id` 引用代理，不再自持代理 URL
- 代理管理页支持连通性测试（探测目标 URL 可按代理配置）
- 代理管理页支持"查看引用"，直接列出使用该代理的渠道
- 完全丢弃旧的 `settings.proxy` 字符串配置（不做数据迁移，用户需在代理管理中重建并重新绑定）

### 1.3 非目标（MVP 之外）

- 渠道绑定多个代理（轮询/故障转移/加权），MVP 内每个渠道最多 1 个代理
- 代理标签/分组
- 代理级别的访问统计与告警
- 普通用户可见的代理配置（代理为管理员专属）

## 2. 架构设计

### 2.1 整体架构

```
┌──────────────────────────────────────────────┐
│  前端 (web/classic)                          │
│  - 代理管理页 /proxy (AdminRoute)             │
│  - 渠道编辑：代理输入改为下拉选择              │
└──────────────┬───────────────────────────────┘
               │ REST
┌──────────────▼───────────────────────────────┐
│  Controller 层 (controller/proxy.go)         │
│  - CRUD / 测试 / 引用查询                     │
└──────────────┬───────────────────────────────┘
               │
┌──────────────▼───────────────────────────────┐
│  Service 层                                  │
│  - service/proxy_test.go 连通性测试           │
│  - relay 请求路径 → 通过 ProxyId 解析 URL     │
└──────────────┬───────────────────────────────┘
               │
┌──────────────▼───────────────────────────────┐
│  Model 层                                    │
│  - model/proxy.go 代理实体 + CRUD             │
│  - model/proxy_cache.go 内存缓存              │
│  - model/channel.go 新增 ProxyId 字段        │
└──────────────────────────────────────────────┘
```

### 2.2 请求路径中的代理解析

```
Relay 请求
  ↓
加载渠道 (含 ProxyId)
  ↓
ProxyId > 0 ?
  ├─ 是 → 查代理缓存
  │       ├─ 代理不存在 → 拒绝请求 (proxy_not_found)
  │       ├─ 代理已禁用 → 拒绝请求 (proxy_disabled)
  │       └─ 正常 → 用代理 URL 构造 HTTP Client
  └─ 否 → 直连
```

**关键约定：** 代理被禁用或删除时**拒绝转发**，不静默降级为直连，避免绕过用户对出口 IP 的意图。

## 3. 数据模型设计

### 3.1 新表 proxies

```go
type Proxy struct {
    Id           int    `json:"id"`
    Name         string `json:"name" gorm:"type:varchar(64);uniqueIndex;not null"`
    Type         string `json:"type" gorm:"type:varchar(16);not null"`         // http / https / socks5
    URL          string `json:"url" gorm:"type:varchar(512);not null"`         // 完整代理 URL
    TestURL      string `json:"test_url" gorm:"type:varchar(512);default:''"`  // 探测目标 URL，空则回退全局默认
    Description  string `json:"description" gorm:"type:varchar(255);default:''"`
    Status       int    `json:"status" gorm:"default:1"`                       // 1=启用 2=禁用
    LastTestTime int64  `json:"last_test_time" gorm:"bigint;default:0"`
    LastTestOK   bool   `json:"last_test_ok" gorm:"default:false"`
    LastTestMsg  string `json:"last_test_msg" gorm:"type:varchar(255);default:''"`
    CreatedTime  int64  `json:"created_time" gorm:"bigint"`
}
```

字段说明：

- `Name`：展示名，全局唯一，用于列表/下拉。
- `Type`：仅用于展示与前端预填 URL scheme；后端仅信任 `URL` 中实际的 scheme。
- `URL`：完整代理地址，格式如 `socks5://user:pass@host:port`。
- `TestURL`：测试连通性时访问的目标 URL。为空时使用全局默认（见 3.3）。
- `Status`：`1=启用`，`2=禁用`。禁用状态代理仍可被查看/编辑，但请求路径拒绝使用。
- `LastTest*`：每次测试成功/失败后回写；用于列表页展示"上次测试状态"。

### 3.2 Channel 表变更

**新增字段：**

```go
type Channel struct {
    // ...existing fields...
    ProxyId *int `json:"proxy_id" gorm:"index;default:0"` // 引用 proxies.id，0/nil 表示不使用代理
}
```

**丢弃字段（不做迁移）：** `dto.ChannelSettings.Proxy` 从代码中移除。所有旧渠道保存的字符串代理**失效**，用户需要在代理管理中重建代理条目并重新绑定渠道。

> 兼容性说明：因为需求明确要求"完全丢弃"，所以不做旧字符串到新代理条目的自动迁移。升级后所有原本走代理的渠道会退化为直连，需要管理员在新版界面重新配置。发布说明中必须明确告知这一破坏性变更。

### 3.3 全局默认测试 URL

存储在 `option` 表：

- key：`DefaultProxyTestURL`
- 默认值：`https://www.google.com/generate_204`
- 后台系统设置页可修改

## 4. 后端 API 设计

所有代理相关接口均在 `AdminRoute` 下。

### 4.1 CRUD

```
GET    /api/proxy/                    列表（分页 + 关键字 name/url/description + status 过滤）
GET    /api/proxy/:id                 详情
POST   /api/proxy/                    新建
PUT    /api/proxy/                    更新（body 含 id）
DELETE /api/proxy/:id                 删除
```

**删除保护：** 若代理被任一渠道引用，返回 `409 Conflict`，body 附带 `referenced_channels: [{id, name}]`，前端提示"请先解绑 N 个渠道"。

### 4.2 精简列表（供渠道下拉使用）

```
GET /api/proxy/options?only_enabled=true
    → [{id, name, type, status}]
```

用于渠道编辑页的下拉数据源。

### 4.3 连通性测试

```
POST /api/proxy/:id/test
    → { ok: bool, latency_ms: int, msg: string }
```

- 后端同步阻塞执行，最大等待 10s
- 结果写回 `LastTestTime/LastTestOK/LastTestMsg`
- 前端在列表页每行提供"测试"按钮触发

### 4.4 查看引用

```
GET /api/proxy/:id/channels
    → [{id, name, type, status}]
```

供代理管理页点击"查看引用"时使用。

### 4.5 错误码约定

| 场景 | HTTP | 说明 |
|------|------|------|
| 名称冲突 | 400 | `proxy name already exists` |
| URL 格式不合法 | 400 | `invalid proxy url` |
| 引用中不允许删除 | 409 | 返回引用列表 |
| 测试超时 | 200 | `ok=false, msg="timeout"` |
| 代理不存在 | 404 | 通用 |

## 5. 请求路径改造

### 5.1 代理解析入口

改造点在渠道 setting 构建处（`relay/channel/api_request.go` 第 489 行附近，以及 `relay/relay_task.go`、`relay/mjproxy_handler.go` 等所有当前读取 `channel.GetSetting().Proxy` 的位置）：

```go
// 旧
if info.ChannelSetting.Proxy != "" {
    client, err = service.NewProxyHttpClient(info.ChannelSetting.Proxy)
}

// 新
proxyURL, err := model.ResolveChannelProxy(channel.ProxyId)
if err != nil {
    return nil, err // proxy_not_found / proxy_disabled
}
if proxyURL != "" {
    client, err = service.NewProxyHttpClient(proxyURL)
}
```

`ResolveChannelProxy` 语义：

- `proxyId == nil || *proxyId == 0` → 返回 `""`（直连）
- 缓存中查不到该 id → 返回错误 `proxy_not_found`
- 代理 `Status != 1` → 返回错误 `proxy_disabled`
- 正常 → 返回代理 URL

### 5.2 代理缓存

参照 `model/channel_cache.go` 新增 `model/proxy_cache.go`：

- 全量加载（代理数量小）
- 提供 `GetProxyById(id int)` / `ResolveChannelProxy(...)` / `InvalidateProxyCache()`
- 代理 CRUD 或测试结果更新后主动失效
- 启动时 `InitProxyCache()` 由 `main.go` 调用

## 6. 连通性测试实现

`service/proxy_test.go` 新增：

```go
func TestProxy(proxyURL, testTarget string, timeout time.Duration) (ok bool, latencyMs int64, msg string)
```

流程：

1. `testTarget` 为空 → 读 option `DefaultProxyTestURL`
2. 用 `NewProxyHttpClient(proxyURL)` 构造 HTTP client（复用现有实现）
3. 记录开始时间 → GET `testTarget` → 记录耗时
4. 判定：
    - 网络/握手失败 → `ok=false, msg=err.Error()`
    - HTTP 状态 < 500 → `ok=true`
    - HTTP 状态 >= 500 → `ok=false, msg="upstream 5xx"`
5. 无论成败，控制器都会把结果写回 `proxies` 表

Timeout 10s 是硬上限；前端 loading 按钮同步等待即可。

## 7. 前端设计（web/classic）

### 7.1 页面与路由

- 新增页面：`pages/Proxy/index.jsx` → 渲染 `components/table/proxies/ProxiesTable.jsx`
- `App.jsx` 加路由 `<Route path='/proxy' element={<AdminRoute><Proxy /></AdminRoute>} />`
- 侧边栏（现有布局组件）在"渠道管理"下方加入"代理管理"菜单项

### 7.2 组件结构

```
components/table/proxies/
├── ProxiesTable.jsx        列表 + 工具栏 + 分页
└── modals/
    ├── EditProxyModal.jsx  新建/编辑
    └── ProxyReferencesModal.jsx  查看引用
```

**列表列：**

| 列 | 说明 |
|----|------|
| Name | 名称 |
| Type | http/https/socks5 |
| URL | 打码显示：`socks5://***:***@host:port` |
| Status | 启用/禁用（切换开关） |
| 最近测试 | 时间 + 成功/失败图标 + tooltip 显示 msg |
| 操作 | 编辑 / 测试 / 查看引用 / 删除 |

**编辑 Modal 字段：** name, type(下拉), url(密码位可"显示"切换), test_url, description, status。

**引用抽屉：** 展示引用渠道的 id、name、type、status，可点击跳转到渠道页并高亮该行。

### 7.3 渠道编辑页改造

`components/table/channels/modals/EditChannelModal.jsx` 中现有 `Form.Input field='proxy'`：

- **删除** 该输入框（不再显示 `proxy` 字符串字段，也不显示"高级折叠区"）
- **新增** `Form.Select field='proxy_id'`：
    - 选项来自 `GET /api/proxy/options?only_enabled=true`
    - 展示 `${name} (${type})`
    - 禁用状态代理不出现在下拉里（避免选中后立即失效）
    - 允许清空（值为 0/null 表示不使用代理）
- 保存渠道时提交 `proxy_id`；不再向 `settings.proxy` 写入任何值
- 同一 Modal 中原 `proxy` 相关 handler（`handleChannelSettingsChange('proxy', ...)`）全部移除

### 7.4 国际化

- `web/classic/src/i18n/locales/*.json`（若使用 flat JSON）新增：
    - 代理管理 / Proxy Management
    - 代理名称 / Proxy Name
    - 代理类型 / Proxy Type
    - 代理地址 / Proxy URL
    - 测试目标 URL / Test Target URL
    - 上次测试 / Last Tested
    - 查看引用 / View References
    - 代理不可用 / Proxy Unavailable
    - 相关提示文本

## 8. 影响文件清单

**后端新增：**

- `model/proxy.go`
- `model/proxy_cache.go`
- `controller/proxy.go`
- `service/proxy_test.go`

**后端修改：**

- `model/channel.go`：新增 `ProxyId *int`，移除对 `settings.proxy` 的读取
- `model/main.go`：`AutoMigrate` 注册 `Proxy{}`，`main.go` 启动时调 `InitProxyCache()`
- `dto/channel_settings.go`：移除 `Proxy string` 字段
- `router/api-router.go`：注册 `/api/proxy/*` 路由
- `relay/channel/api_request.go`、`relay/relay_task.go`、`relay/mjproxy_handler.go`、`relay/channel/gemini/relay-gemini.go`、`relay/channel/task/*/adaptor.go`：把 `channel.GetSetting().Proxy` 全部替换为 `model.ResolveChannelProxy(channel.ProxyId)` 的调用（保留仍需 proxy 字符串的下游 API 签名不变）
- `common/constants.go` 或对应 option 初始化文件：新增 `DefaultProxyTestURL` 默认值

**前端新增：**

- `web/classic/src/pages/Proxy/index.jsx`
- `web/classic/src/components/table/proxies/ProxiesTable.jsx`
- `web/classic/src/components/table/proxies/modals/EditProxyModal.jsx`
- `web/classic/src/components/table/proxies/modals/ProxyReferencesModal.jsx`

**前端修改：**

- `web/classic/src/App.jsx`：新增路由
- 侧边栏菜单组件：新增"代理管理"入口
- `web/classic/src/components/table/channels/modals/EditChannelModal.jsx`：移除 `proxy` 输入，新增 `proxy_id` 下拉
- `web/classic/src/i18n/locales/*.json`：新增翻译

## 9. 数据库兼容性

- 所有字段类型均为跨库通用（`varchar` / `bigint` / `boolean` / `int`），符合 CLAUDE.md Rule 2
- 无原始 SQL，全部走 GORM 抽象
- `ProxyId` 字段使用 `default:0`，SQLite 上通过 `ALTER TABLE ADD COLUMN` 添加

## 10. 安全性

- 所有代理管理 API 强制 `AdminRoute`
- 代理 URL 中的 password 部分在前端列表打码显示，仅编辑 Modal 内可"显示明文"
- 日志（`common.SysLog`）中不打印完整代理 URL，只打印 `name` 与 `id`
- 代理测试仅走后端发起（不暴露给浏览器），避免代理认证信息泄露

## 11. 测试计划

- 单测：`model/proxy_test.go` 覆盖 CRUD + 缓存失效；`service/proxy_test_internal_test.go` 覆盖成功/超时/5xx 三条路径
- 集成：SQLite / MySQL / PostgreSQL 各跑一次迁移 + 基本 CRUD
- 端到端（手工）：
    1. 新建代理 → 测试连通性 → 绑定到渠道 → 发一个 chat 请求走该代理
    2. 禁用代理 → 请求应报错 `proxy_disabled`
    3. 删除被引用代理 → 返回 409 + 引用列表
    4. 修改代理 URL → 已引用渠道立即生效（缓存失效验证）
