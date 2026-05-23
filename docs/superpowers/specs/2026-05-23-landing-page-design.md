# Landing Page (着陆页) — 设计文档

**日期**: 2026-05-23
**作者**: 设计协作 via Claude
**范围**: v1, web/classic 前端纯静态着陆页, 后端零改动

---

## 1. 背景与目标

dashboard 当前首页 (`/`) 由 `pages/Home/` 提供, 内容偏向"已登录用户的状态面板"。
需要一个面向新访问者的着陆页, 用于介绍平台能力 (文生图 / 文生视频 / 模型聚合),
并把用户引导到对应入口。

v1 目标:

- 新增独立路由 `/landing`, 用户在浏览器直接访问该路径即可看到
- 暗色 + 节制辉光的 SaaS 风, 接近 Linear / Vercel 调性
- 包含 5 个 section: Hero / 能力卡片 / 数据带 / 模型清单 / 使用步骤
- 模型清单数据来自 `/api/user/models` (登录后自动透出用户可用模型),
  其余统计数字写死
- 文生视频卡片点 "立即体验" 跳 `/console/video-playground`
- 文生图卡片显示 "敬请期待" (按钮存在但点击无反应)
- 8 个语言的 i18n 翻译跟齐

**非目标 (YAGNI)**:

- 不动 `pages/Home/` 任何内容; 不接管 `/` 路由
- 不做系统设置开关 / 不做 "首页样式" 配置项; 后端零改动
- 不引入 footer / 关于我们 / 价格套餐 section
- 不嵌入上游厂商 logo (避免商标问题), 用首字母彩色圆代替
- 不做动画 / 视差 / 自动轮播
- 不做 SEO meta (dashboard 内部页, 不面向搜索引擎)
- web/default 主题不实现 (项目惯例: 当前阶段只动 web/classic)
- 不做文生图 Playground (按钮 disabled, 后续单独立项)

## 2. 总体架构

纯前端静态页面, 单一新增 Route, 复用现有 Layout (顶部 navbar / 侧边栏保留)。

```
web/classic/src/
  ├── pages/Landing/
  │   ├── index.jsx                   — Landing 根组件, 编排各 section
  │   ├── useLandingData.js           — 拉 /api/user/models + 分类 + fallback
  │   └── sections/
  │       ├── Hero.jsx                — 主标题 / 副标题 / CTA 按钮
  │       ├── CapabilityCards.jsx     — 文生图 + 文生视频两张卡
  │       ├── StatsBand.jsx           — 三组占位数字
  │       ├── ModelGrid.jsx           — 已接入模型网格
  │       └── Steps.jsx               — 三步使用流程
  └── App.jsx                         — 新增一行 <Route path='/landing' .../>

web/classic/src/i18n/locales/*.json   — 8 个语言文件追加 ~20 个 key
```

**为什么这么拆**:

- 每个 section 独立文件 — 单独读 / 单独改 / 不互相牵连; 将来想下掉某一段直接删 import
- `useLandingData` 隔离网络请求 — section 组件保持纯渲染, 易测、易缓存
- Landing 挂在现有 Layout 内 — 复用 navbar / sidebar / theme provider, 不需要再做
  一套独立的外壳
- 完全不动后端 — option 表、StatusContext、设置 UI 全部砍掉; "替换首页" 等用户
  真正需要时, 自己改 `App.jsx` 里 `path='/'` 那行的 element 即可

## 3. 路由切换机制

`App.jsx` 唯一改动: 新增一行 Route 注册, 不替换任何已有路由。

```jsx
// 新增
<Route
  path='/landing'
  element={<Suspense fallback={<Loading />}><Landing /></Suspense>}
/>
```

`/` 仍指向原 `<Home />`, 完全保留。"替换首页" 不在 v1 范围 — 哪天确实要替换,
管理员或开发者自行把 `element={<Home />}` 改成 `element={<Landing />}` 即可,
该操作不属于本次工程范围。

**后端**: 一行不改。`option` 表、`StatusContext`、`/api/status`、系统设置 UI
全部砍掉。

## 4. 视觉设计

调性: 暗色 + 节制辉光的 SaaS 风格, 接近 Linear / Vercel 那种"低噪音科技感"。

### 4.1 色板

| 用途             | Tailwind class            | 备注                                         |
| ---------------- | ------------------------- | -------------------------------------------- |
| 全局背景         | `bg-slate-950` (#0a0a0a)  | 整个 Landing 页面底色                        |
| 卡片底           | `bg-zinc-900`             | 能力卡 / 模型卡内部                          |
| 卡片描边         | `border-zinc-800`         | 1px, hover 时切到 `border-indigo-500/40`     |
| 文字主           | `text-zinc-200`           | 标题 / 主要正文                              |
| 文字副           | `text-zinc-400`           | 副标题 / 描述                                |
| 文字暗           | `text-zinc-500`           | 标签 / 数据带 label                          |
| 强调色 (渐变)    | `from-indigo-400 to-cyan-300` | 仅用于: 主标题局部 span / 数据带数字 / 主 CTA hover |
| Hero 辉光        | radial-gradient indigo 20% 透明 | 离视觉中心稍远, 不喧宾夺主              |

### 4.2 全局规则

- 字体: 继承现有 dashboard 字体栈, 不引入新字体
- 数字: `tabular-nums` + `font-feature-settings: "ss01"` (如可用) 让数字齐整
- 圆角: `rounded-2xl` 卡片 / `rounded-full` 数字徽章
- 间距: section 之间 `py-20`, 卡片 padding `p-8`
- 响应式: `<lg` Hero 标题降到 4xl, 能力卡 1 列, ModelGrid 2 列

## 5. 各 Section 设计

### 5.1 Hero

高度约 70vh, 内容居中。

```
   ┌─────────────────────────────────────────────┐
   │                                             │
   │   在一个网关里, 调度全部主流 AI 模型        │  ← 5xl/6xl, zinc-100
   │   ────────────────────────────────────      │     "全部主流 AI 模型" 渐变 span
   │                                             │
   │   new-api 把文生图、文生视频和聊天补全聚合到│  ← lg, zinc-400
   │   统一接口, 让团队不用为每家模型重写一份调用│     最宽 720px
   │                                             │
   │   ┌────────────┐  ┌────────────┐            │
   │   │ 立即开始 → │  │ 查看文档   │            │  ← 主按钮 indigo, 次按钮 ghost
   │   └────────────┘  └────────────┘            │
   │                                             │
   └─────────────────────────────────────────────┘
                  ░░ 辉光 (radial indigo 20%) ░░
```

- 主标题: 默认 `text-zinc-100 text-5xl lg:text-6xl font-semibold`,
  其中 "全部主流 AI 模型" 单独包 `<span>` 套渐变文字
- 副标题: `text-zinc-400 text-lg` 最宽 `max-w-3xl`
- 主 CTA "立即开始": 视觉存在 (indigo 主按钮), **不挂 onClick / 不跳转任何地方**;
  点击无反应。视觉上保持正常 CTA 样式 (不是灰态), 仅作装饰 / 占位, 后续如需接路由
  再单独立项
- 次 CTA "查看文档": 同上, 视觉存在 (ghost 次按钮), 不挂 onClick, 点击无反应
- 辉光: 绝对定位放在 Hero 底部, `pointer-events-none`, 不影响交互

### 5.2 CapabilityCards (文生图 + 文生视频)

紧贴 Hero 下方, 两张卡片并排, `<lg` 改 1 列。

```
   ┌─────────────────────────┐  ┌─────────────────────────┐
   │ ◆                       │  │ ▶                       │
   │ 文生图                  │  │ 文生视频                │
   │                         │  │                         │
   │ 一句 Prompt, 触达多家   │  │ Doubao Seedance、Sora、 │
   │ 图像生成模型, 自动选路  │  │ Kling 等模型统一接入,   │
   │ 与计费。                │  │ 提交任务即可异步出片。  │
   │                         │  │                         │
   │ [敬请期待]              │  │ [立即体验 →]            │
   └─────────────────────────┘  └─────────────────────────┘
   (按钮 disabled, 灰态)         (按钮 indigo,
   (cursor-default, no onClick)   跳 /console/video-playground)
```

- 卡片样式: `bg-zinc-900 border border-zinc-800 rounded-2xl p-8`,
  hover 时 `border-indigo-500/40`, 不做复杂阴影动画
- 图标: 用 `lucide-react` 已经在用的 `Image`, `Video` / `Film`
- "敬请期待" 按钮: `disabled` 属性 + `cursor-default` + 不挂 onClick handler
- "立即体验" 按钮: `useNavigate('/console/video-playground')` 跳到现有 Playground

### 5.3 StatsBand (数据带)

一条水平区, 纯文字, 三列等宽。

```
   ───────────────────────────────────────────────────────────
       40+                99.9%               1ms+
       接入上游模型供应商  网关请求可用性     路由层平均开销
   ───────────────────────────────────────────────────────────
```

- 数字: 渐变文字 + `text-5xl font-semibold tabular-nums`
- 标签: `text-sm text-zinc-500`
- 上下: 各一条 `border-zinc-800/60` 细线
- 数字全部 hardcode 在组件里 (v1 不读后端统计); 后续如要接真实统计, 单独立项

### 5.4 ModelGrid (已接入模型)

```
   已接入模型
   接入超过 {{n}} 家模型, 覆盖图像、视频、对话等场景。

   ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐
   │ D        │ │ S        │ │ K        │ │ G        │
   │ Doubao.. │ │ Sora     │ │ Kling    │ │ Gemini   │
   │ 视频     │ │ 视频     │ │ 视频     │ │ 文本/图像│
   └──────────┘ └──────────┘ └──────────┘ └──────────┘
   ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐
   │ ...      │
                       ⋯  最多 12 条  ⋯
                       [ 查看全部 → ]
```

**数据流**:

1. `useLandingData` hook 调 `API.get('/api/user/models')`
2. 返回的 model name 列表去重
3. 按关键字简单分类:
   | 类别 | 关键字 (case insensitive) |
   |------|---------------------------|
   | 图像 | `image`, `flux`, `sd`, `midjourney`, `dall` |
   | 视频 | `video`, `sora`, `kling`, `seedance`, `runway`, `pika` |
   | 文本 | 其余全部归入 |
4. 取前 12 条渲染
5. 接口失败 / 未登录 / 返回为空时, fallback 到组件内写死的 6 条占位
   (覆盖 3 个类别各 2 个, 名字用通用占位如 "Image-A", "Video-A", "Text-A")

**单卡样式**:

- 尺寸: 固定宽度 (响应式 grid 决定), `aspect-ratio` 约 4:3
- 左上首字母圆: `w-10 h-10 rounded-full bg-indigo-500/15 text-indigo-300`,
  字符取 model name 首字母大写
- 主行: model name 单行 `truncate`
- 副行: 类别 tag `text-xs text-zinc-500`
- 不放上游厂商 logo (商标风险)

**底部**: 一个 "查看全部 →" 链接, `useNavigate('/pricing')`
(项目内已有 `/pricing` 路由展示所有模型与价格)

### 5.5 Steps (三步引导)

```
       ①                    ②                    ③
   ┌────────┐ ──────── ┌────────┐ ──────── ┌────────┐
   │ 选择   │           │ 编写   │           │ 查看   │
   │ 模型   │           │ Prompt │           │ 结果   │
   │        │           │        │           │        │
   │ 按任务 │           │ 在 Play│           │ 异步任 │
   │ 类型筛 │           │ ground │           │ 务可在 │
   │ 选可用 │           │ 里调参 │           │ 控制台 │
   │ 模型,  │           │ 试跑,  │           │ 追踪进 │
   │ 平台自 │           │ 或直接 │           │ 度,命  │
   │ 动匹配 │           │ 调用统 │           │ 中失败 │
   │ 最优渠 │           │ 一 API │           │ 自动重 │
   │ 道.    │           │ .      │           │ 试.    │
   └────────┘           └────────┘           └────────┘
```

- 数字徽章: `w-10 h-10 rounded-full border border-indigo-500/40 text-indigo-300`
- 步骤间细横线: `border-zinc-800`, `<lg` 隐藏 (移动端堆叠成竖列)
- 步骤标题 + 描述, 都走 i18n

## 6. i18n key 清单

每个语言文件 (en / zh-CN / zh-TW / zh / fr / ja / ru / vi) 追加以下 key,
中文是 key 本身, 其它语言按语义翻译:

| key                                                                                | 备注                              |
| ---------------------------------------------------------------------------------- | --------------------------------- |
| `在一个网关里,调度全部主流 AI 模型`                                                | Hero 主标题                       |
| `new-api 把文生图、文生视频和聊天补全聚合到统一接口,让团队不用为每家模型重写一份调用。` | Hero 副标题                       |
| `立即开始`                                                                         | Hero 主 CTA                       |
| `查看文档`                                                                         | Hero 次 CTA                       |
| `一句 Prompt,触达多家图像生成模型,自动选路与计费。`                                | 文生图卡片描述                    |
| `Doubao Seedance、Sora、Kling 等模型统一接入,提交任务即可异步出片。`               | 文生视频卡片描述                  |
| `立即体验`                                                                         | 文生视频 CTA                      |
| `接入上游模型供应商`                                                               | 数据带 ① 标签                     |
| `网关请求可用性`                                                                   | 数据带 ② 标签                     |
| `路由层平均开销`                                                                   | 数据带 ③ 标签                     |
| `已接入模型`                                                                       | ModelGrid 区块标题                |
| `接入超过 {{n}} 家模型,覆盖图像、视频、对话等场景。`                               | ModelGrid 描述, `n` 是当前条目数  |
| `选择模型`                                                                         | 步骤 ① 标题                       |
| `按任务类型筛选可用模型,平台自动匹配最优渠道。`                                    | 步骤 ① 描述                       |
| `编写 Prompt`                                                                      | 步骤 ② 标题                       |
| `在 Playground 里调参试跑,或直接调用统一 API。`                                    | 步骤 ② 描述                       |
| `异步任务可在控制台追踪进度,命中失败自动重试。`                                    | 步骤 ③ 描述                       |
| `图像`                                                                             | ModelGrid 分类 tag                |
| `视频`                                                                             | ModelGrid 分类 tag                |
| `文本`                                                                             | ModelGrid 分类 tag                |

**复用现有 key (不需要重新加)**:

- `文生图` / `文生视频` — 已存在
- `敬请期待` — 多数 locale 已存在
- `查看结果` — 已存在
- `查看全部` — 已存在

## 7. 实现拆步

按下面顺序执行, 每一步都是一个独立的小变更:

1. **文件骨架**: 新建 `pages/Landing/index.jsx` 和 5 个 section 文件 +
   `useLandingData.js`, 全部先放占位 `return null`, 跑通 import 链
2. **路由**: `App.jsx` 加 `<Route path='/landing'>`, 浏览器访问 `/landing`
   看到空白 Landing 页 (顶部 navbar 仍在)
3. **i18n**: 8 个 locale 各追加 ~20 个 key. JSON 校验通过 (用之前用过的
   `node -e "JSON.parse(...)"` 一遍)
4. **Hero**: 静态文字 + 渐变 span + 两个按钮, **不挂 onClick**, 单独验收
5. **CapabilityCards**: 两张卡, 文生图按钮 disabled, 文生视频按钮跳
   `/console/video-playground`
6. **StatsBand**: 三个写死数字 + 渐变文字
7. **ModelGrid + hook**: 实现 `useLandingData` (拉接口 + 去重 + 分类 + 取前 12 +
   fallback 6 条), 渲染卡片网格, "查看全部" 链接
8. **Steps**: 三步纯静态
9. **响应式**: 把每个 section 的 `<lg:` 断点都过一遍, 移动端检查
10. **本地验证**: `bun run dev` → 浏览器访问 `/landing`, 切语言、缩放窗口、
    断网模拟 fallback 路径

## 8. 风险与回退

| 风险                                          | 影响                          | 应对                                     |
| --------------------------------------------- | ----------------------------- | ---------------------------------------- |
| `/api/user/models` 未登录时 401               | ModelGrid 空                  | hook 内 catch → fallback 到 6 条占位     |
| 用户可用模型为空                              | 网格空                        | 同上, fallback 占位                      |
| 上游品牌词被认为是商标使用                    | 文案合规                      | 仅用文字提及, 不嵌 logo; 描述里出现的"Doubao Seedance / Sora / Kling" 都是公开模型名引用, 没有 logo 资源 |
| 用户改 App.jsx 把 / 替换成 Landing 后想回退   | 主页打不开                    | 把 element 改回 `<Home />` 即可; 本次未提供"开关", 改路由需自己走 |

## 9. 验收清单

- [ ] 访问 `/landing` 渲染完整 5 个 section
- [ ] 切换语言 (zh-CN / en / 其余 6 个) Landing 文案全部翻译跟齐
- [ ] 浏览器宽度 < 1024 时, 各 section 切到移动端布局
- [ ] Hero "立即开始" / "查看文档" 两个按钮点击均无反应, 视觉保持正常 CTA 样式
- [ ] 文生视频卡片 "立即体验" 跳 `/console/video-playground`
- [ ] 文生图卡片 "敬请期待" 点击无反应、视觉灰态
- [ ] ModelGrid 在登录态下展示真实模型, 模拟接口失败时展示 6 条占位
- [ ] `/` 路由仍然是原 `<Home />`, 内容未变
- [ ] 后端无任何代码变化, `git diff -- backend/ controller/ middleware/ router/ model/ service/` 为空
