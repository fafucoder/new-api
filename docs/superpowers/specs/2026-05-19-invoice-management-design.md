# 发票管理 (Invoice Management) — 设计文档

**日期**: 2026-05-19
**作者**: 设计协作 via Claude
**范围**: v1, web/classic 前端 + 后端 API + stub 第三方 provider

---

## 1. 背景与目标

用户在 new-api 充值消费后, 需要按充值金额开具发票用于报销 / 入账。
v1 提供:

- 用户在 web/classic 主题下申请开票
- 区分**个人 / 企业**两种申请人, 企业需填**统一社会信用代码**
- 申请抬头 / 接收邮箱 / 发票类型每次填写, 不在用户档案中持久化
- 管理员后台审核或自动开票(可配置)
- 第三方开票走抽象接口, v1 仅注册 stub provider; 后续可挂载讯汇 / 腾讯电子发票 / 百望等
- 系统配置: 启用开关、最低开票额度、是否需要人工审核、provider 选择

**非目标 (YAGNI)**:

- 不做发票作废 / 红冲
- 不做按 topup 明细勾选, 仅"全部未开票余额"一次性申请
- 不存抬头模板, 每次重填
- 不引入消息队列, 走现有协程异步模式
- web/default 主题 v1 不实现
- 不接真实第三方, stub 即可
- 不暴露"重新发送邮件"按钮 (管理员需要时直接操作)

## 2. 总体架构

仿现有 `balance_alert` 模式: Router → Controller → Service → Model 单表。

```
router/api-router.go
  ├── /api/invoice/*           (登录用户)
  └── /api/invoice/admin/*     (管理员)

controller/invoice.go          — handler
service/invoice/
  ├── service.go               — Apply/Issue/Reject/List/Summary 编排
  ├── provider.go              — InvoiceProvider 接口 + DTO
  ├── stub_provider.go         — 默认实现
  └── registry.go              — Register(name, factory) / Get(name)
model/invoice.go               — Invoice 实体 + CRUD + 金额聚合
setting/operation_setting/invoice_setting.go — 配置
dto/invoice.go                 — Request/Response

web/classic/src/
  ├── pages/Invoice/index.jsx          — 用户页
  ├── pages/InvoiceAdmin/index.jsx     — 管理员页
  └── pages/Setting/Operation/SettingsInvoice.jsx — 配置面板
```

## 3. 数据模型

**表**: `invoices` (新建, GORM AutoMigrate)

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | int, PK | |
| `user_id` | int, index | 申请人 |
| `applicant_type` | varchar(16) | `personal` / `enterprise` |
| `title` | varchar(128) | 发票抬头 |
| `tax_id` | varchar(32) | 统一社会信用代码; personal 时为空 |
| `email` | varchar(128) | 接收邮箱 |
| `invoice_type` | varchar(16) | `vat_normal` / `vat_special` (专票仅允许 enterprise) |
| `amount` | decimal(16,4) | 申请金额快照, USD; 等于提交时的"未开票余额" |
| `status` | varchar(16), index | `pending` / `issuing` / `issued` / `rejected` |
| `reject_reason` | varchar(256) | 拒绝原因, 仅 rejected 有值 |
| `reviewer_id` | int | 处理该申请的管理员 (0 表示自动) |
| `provider` | varchar(32) | 使用的 provider key, eg `stub`、`nuonuo` |
| `provider_invoice_no` | varchar(64) | 第三方回执号 |
| `provider_pdf_url` | varchar(512) | 发票 PDF 链接 |
| `provider_raw` | text | 第三方完整响应或最近一次错误, JSON |
| `applied_at` | bigint | 提交申请时间 |
| `issued_at` | bigint | 开票成功时间 |
| `created_time` | bigint, autoCreateTime | |
| `updated_time` | bigint, autoUpdateTime | |

**索引**:
- `idx_invoices_user_id` (user_id)
- `idx_invoices_status` (status)
- `idx_invoices_user_status` (user_id, status) — 加速 in-flight 检测和金额聚合

**派生量**(实时计算, 不冗余):

- **已开票金额** = `SUM(amount) WHERE user_id=? AND status IN ('pending','issuing','issued')`
  - `pending` 也占额度, 防止用户重复申请
  - `rejected` 不占
- **可开票余额** = `SUM(topups.money WHERE user_id=? AND status='success')` − **已开票金额**

**字段约束**:

- `applicant_type='personal'` 时 `tax_id` 必须空、`invoice_type` 必须 `vat_normal`
- `applicant_type='enterprise'` 时 `tax_id` 必填且非空
- 同一用户最多一条 `pending` 或 `issuing` (service 层加锁 + 二次检查实现, 跨 DB)
- `amount` 必须 ≥ `MinimumAmount` (从 settings 读)

## 4. 后端

### 4.1 Provider 抽象 (`service/invoice/provider.go`)

```go
type IssueRequest struct {
    InvoiceID     int
    UserID        int
    ApplicantType string  // personal | enterprise
    Title         string
    TaxID         string
    Email         string
    InvoiceType   string  // vat_normal | vat_special
    Amount        float64 // USD
}

type IssueResult struct {
    ProviderInvoiceNo string
    PDFURL            string
    RawResponse       string  // 原样 JSON 留痕
}

type InvoiceProvider interface {
    Name() string
    Issue(ctx context.Context, req *IssueRequest) (*IssueResult, error)
}
```

### 4.2 Registry (`service/invoice/registry.go`)

```go
var providers = map[string]func() InvoiceProvider{}

func Register(name string, factory func() InvoiceProvider) { providers[name] = factory }
func Get(name string) (InvoiceProvider, error) { ... }
```

`init()` 时注册 `stub`。后续接真实 provider 时新增一个文件即可。

### 4.3 Stub Provider (`service/invoice/stub_provider.go`)

- 仅打 `logger.Info`, 等待 200ms 模拟网络
- `ProviderInvoiceNo` = `"STUB-" + invoice_id + "-" + timestamp`
- `PDFURL` = 空字符串 (前端"下载 PDF"按 url 是否非空显示)
- `RawResponse` = `{"provider":"stub","ts":...}`
- 永远成功

### 4.4 Service 业务编排 (`service/invoice/service.go`)

| 方法 | 调用方 | 行为 |
|---|---|---|
| `Apply(userID, form)` | 用户 controller | 见 §4.4.1 |
| `Issue(invoiceID, reviewerID)` | 管理员 controller / Apply 异步分支 | 见 §4.4.2 |
| `Reject(invoiceID, reviewerID, reason)` | 管理员 controller | status 必须 pending; 置 `rejected`, 写 reason / reviewer_id |
| `List(userID, page, pageSize)` | 用户 controller | 仅查自己, 按 applied_at desc |
| `AdminList(filter, page, pageSize)` | 管理员 controller | 支持 status / user_id / 时间区间 |
| `Summary(userID)` | 用户 controller | 返回可开票余额 / 已开票总额 / 申请中金额 / 最低额度 / Enabled 标志 / RequireManualReview 标志 |

#### 4.4.1 `Apply` 流程

1. 校验 `settings.Enabled`, 否则返回 `ErrFeatureDisabled`
2. 字段级校验 (title / email 必填, applicant_type 合法, 个人/企业的 tax_id / invoice_type 组合合法)
3. 算可开票余额, 必须 ≥ `MinimumAmount`, 否则 `ErrAmountBelowMinimum`
4. 在事务里:
   - `SELECT ... FOR UPDATE` 锁 user_id 维度 (用 `users.id`; SQLite 退化为整表锁, 可接受)
   - 二次检查无 in-flight
   - 写一条 `Invoice{status:'pending', amount: 当前可开票余额, ...}`
5. 事务外: 如果 `RequireManualReview=false`, 用 `go` 协程跑 `Issue(invoiceID, 0)`; 失败仅记 log
6. 返回新 invoice id

#### 4.4.2 `Issue` 流程 (乐观锁状态机)

1. 加载 invoice; status 必须 `pending`, 否则 `ErrInvalidStatus`
2. `UPDATE invoices SET status='issuing' WHERE id=? AND status='pending'`; 若影响行数 = 0, 返回 "状态已变更"
3. 调 `provider.Issue(ctx, req)`
4. 成功:
   - 置 `status='issued'`, 写 `provider_invoice_no` / `provider_pdf_url` / `provider_raw` / `issued_at` / `reviewer_id`
   - 走现有 `service.user_notify` 邮件通道发通知, i18n 模板内联 PDF 链接 (不附件)
5. 失败:
   - 退回 `status='pending'`, 写 `provider_raw = {"error":"...","at":ts}`
   - `logger.Error` 记录原因, 不抛给用户

### 4.5 路由 (`router/api-router.go`)

```go
invoiceRoute := apiRouter.Group("/invoice")
invoiceRoute.Use(middleware.UserAuth())
{
    invoiceRoute.GET("/summary", controller.GetInvoiceSummary)
    invoiceRoute.GET("/list", controller.GetInvoiceList)
    invoiceRoute.POST("/apply", controller.PostInvoiceApply)
}

invoiceAdminRoute := apiRouter.Group("/invoice/admin")
invoiceAdminRoute.Use(middleware.AdminAuth())
{
    invoiceAdminRoute.GET("/list", controller.GetInvoiceAdminList)
    invoiceAdminRoute.POST("/:id/issue", controller.PostInvoiceIssue)
    invoiceAdminRoute.POST("/:id/reject", controller.PostInvoiceReject)
}
```

### 4.6 配置 (`setting/operation_setting/invoice_setting.go`)

```go
type InvoiceSetting struct {
    Enabled             bool    `json:"enabled"`               // 默认 false
    MinimumAmount       float64 `json:"minimum_amount"`        // 默认 50.0, USD
    RequireManualReview bool    `json:"require_manual_review"` // 默认 true
    Provider            string  `json:"provider"`              // 默认 "stub"
}
```

通过 `config.GlobalConfig.Register("invoice_setting", &invoiceSetting)` 注册, OptionMap 落库, 与 `monitor_setting` 同模式。

### 4.7 JSON / 数据库兼容

- 所有 marshal/unmarshal 走 `common.Marshal` / `common.UnmarshalJsonStr` (CLAUDE.md Rule 1)
- 全部 GORM, 不用原生 SQL; `decimal(16,4)` 兼容三库 (CLAUDE.md Rule 2)
- `provider_raw` 用 `text` 而非 `jsonb`

## 5. 前端 (web/classic)

### 5.1 路由 + 侧边栏

- 用户端: 钱包管理分组下加 `发票管理` → `/invoice`
- 管理员端: 系统管理分组下加 `发票审核` → `/invoice-admin`
- 配置面板: `Setting/Operation` 已有 Settings 折叠面板里追加 `SettingsInvoice`

### 5.2 用户页 `/invoice` (`pages/Invoice/index.jsx`)

布局:

1. **余额卡片** (顶部)
   - 可开票余额 / 已开票总额 / 申请中金额
   - 最低开票额 (来自 `/api/invoice/summary`)
   - 按钮 `申请开票`:
     - settings.Enabled=false 或 可开票余额 < Minimum 时置灰
     - tooltip 解释原因
2. **历史 Table**
   - 列: 申请时间 / 类型 / 抬头 / 金额 / 状态 Tag / 开票号 / PDF / 操作
   - 状态 Tag 颜色: pending 灰 / issuing 蓝 / issued 绿 / rejected 红
   - PDF 列仅 issued 显示下载图标, 点击新窗口打开
3. **申请 Modal** (点"申请开票"打开)
   - Radio 个人 / 企业
   - Input 抬头 (必填)
   - Input 统一社会信用代码 (仅 enterprise 可见, 必填)
   - Input 邮箱 (默认填用户邮箱, 可改)
   - Select 发票类型: 普通发票 / 增值税专用发票 (personal 时锁定普通)
   - 只读金额: 显示当前可开票余额, 说明 "本次将申请开具全部未开票余额的发票"
   - 提交: `POST /api/invoice/apply`, 关闭 Modal, 刷新

### 5.3 管理员页 `/invoice-admin` (`pages/InvoiceAdmin/index.jsx`)

- 顶部筛选: 状态多选 + 用户搜索 + 申请时间区间
- Table 列: ID / 用户 / 类型 / 抬头 / 税号 / 邮箱 / 发票类型 / 金额 / 状态 / 申请时间 / 操作
- 操作列:
  - pending: `开票` (主按钮 Popconfirm) / `拒绝` (Modal 填 reason)
  - 其它: 仅查看
- 详情抽屉: 显示完整字段 + `provider_raw` (有错时高亮)

### 5.4 配置面板 (`SettingsInvoice.jsx`)

参照 `SettingsMonitoring.jsx`:

- Switch: 启用发票管理
- Switch: 提交后需要管理员审核 (关闭则自动开票)
- Number: 最低开票额度 (USD, ≥ 0)
- Select: 开票服务商 (v1 仅 `stub`)
- `保存设置` 按钮 → `POST /api/option/` (走现有 OptionMap)

### 5.5 i18n

- 七个 locale 文件 (en/zh-CN/zh-TW/fr/ja/ru/vi) 同步添加新 key
- 用户/管理员可见文本全部走 `t()` 包装
- 后端邮件模板国际化走 `i18n/translations/{en,zh}/` (go-i18n)

## 6. 边界与错误处理

| 场景 | 行为 |
|---|---|
| `Enabled=false` | 用户端 `/api/invoice/*` 全部返回 403 + i18n 错误信息; 申请按钮在前端置灰 |
| 申请金额 < `MinimumAmount` | service 层拒绝, 返回 "未开票余额 $X 低于最低开票额度 $Y" |
| 存在 in-flight (pending/issuing) | 拒绝, "您有未完成的发票申请, 请等待处理或联系管理员" |
| 抬头/邮箱/税号校验失败 | service 层校验; 前端 form 也做基础校验 |
| 个人选了专票 / 个人填了税号 | service 层拒绝 |
| Provider.Issue 返回 error | invoice 退回 `pending`, 错误写入 `provider_raw`, 不抛给用户 |
| Provider.Issue panic | recover, 等同 error 处理, 记 `logger.Error` |
| 两个 admin 同时点同一条 pending 的 issue | 乐观锁保证只有一个成功, 另一个返回 "状态已变更, 请刷新" |
| 邮件发送失败 | 不影响 issued 状态, 仅记 log; 管理员页可见 PDF 链接 |
| settings.Enabled 在申请途中被关闭 | 已 pending 的继续走完, 新申请被拒 |

## 7. 测试

- `model/invoice_test.go`
  - CRUD
  - `SumInvoicedAmount` 按 user / 状态过滤正确
  - in-flight 检测的 race-free 性 (单线程模拟)
- `service/invoice/service_test.go`
  - `Apply` 分支: 关闭 / 低于阈值 / personal+专票 / personal+税号 / 重复申请 / 正常
  - `Issue` 状态机: 乐观锁 / provider 成功 / provider 失败回退 / panic 回退
  - `Reject` 状态合法性
- `service/invoice/stub_provider_test.go`
  - 验证返回结构与 invoice id 关联

测试库: SQLite 内存, 与现有 `model/*_test.go` 一致。

## 8. 文件清单 (实施时新增 / 修改)

**新增**:
- `model/invoice.go`, `model/invoice_test.go`
- `service/invoice/{provider,stub_provider,registry,service}.go` + 对应 `_test.go`
- `controller/invoice.go`
- `dto/invoice.go`
- `setting/operation_setting/invoice_setting.go`
- `web/classic/src/pages/Invoice/index.jsx`
- `web/classic/src/pages/InvoiceAdmin/index.jsx`
- `web/classic/src/pages/Setting/Operation/SettingsInvoice.jsx`
- `i18n/translations/{en,zh}/invoice.toml` (邮件模板)

**修改**:
- `router/api-router.go` — 路由注册
- `model/main.go` — AutoMigrate 加 `&Invoice{}`
- `web/classic/src/App.jsx`, `hooks/common/useSidebar.js`, `components/layout/SiderBar.jsx` — 侧边栏 + 路由
- `web/classic/src/i18n/locales/*.json` (7 个文件) — 翻译 key

## 9. 推迟项

- 接真实 provider (新增一个 `nuonuo_provider.go`, 不动既有代码)
- 作废 / 红冲 (新增状态 `voided`, 反向冲账)
- 抬头模板 (`invoice_titles` 表 + 用户档案集成)
- web/default 主题端口
- 按 topup 明细勾选
- 邮件附件代替链接 (PDF 大附件性能)
