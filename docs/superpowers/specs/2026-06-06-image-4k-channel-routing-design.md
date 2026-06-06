---
title: 图片生成4K渠道路由设计
date: 2026-06-06
author: Claude Code
status: approved
---

# 图片生成4K渠道路由设计

## 概述

为 `/v1/images/generations` 接口实现基于图片尺寸的渠道路由功能。系统将根据请求的图片尺寸判断是否需要4K渠道，并在渠道选择时过滤掉不支持4K的渠道。

## 背景

部分上游渠道不支持高分辨率（4K）图片生成。为避免将4K请求路由到不支持的渠道导致失败，需要：

1. 允许管理员标记渠道是否支持4K
2. 系统可配置如何定义1K（普通分辨率）图片的标准
3. 请求时根据图片尺寸自动路由到合适的渠道

## 需求

### 功能需求

1. **系统配置**：管理员可在系统设置中配置1K图片的判断标准
2. **渠道配置**：管理员可在渠道编辑页面标记渠道是否支持4K
3. **自动路由**：图片生成请求根据尺寸自动选择支持4K或普通分辨率的渠道

### 非功能需求

1. 向后兼容：未配置4K支持的渠道默认视为支持所有尺寸
2. 性能：尺寸判断逻辑不影响请求性能
3. 灵活性：支持多种尺寸判断模式，适配不同业务场景

## 设计

### 1. 尺寸判断模式

支持三种判断模式：

| 模式 | 说明 | 示例 |
|------|------|------|
| `total_pixels` | 按总像素数判断 | 1024x1024 = 1,048,576 像素 |
| `min_edge` | 按最小边长判断 | 1024x1536 的最小边是 1024 |
| `max_edge` | 按最大边长判断 | 1024x1536 的最大边是 1536 |

管理员配置：
- **模式**：选择上述三种模式之一
- **阈值**：小于等于此值为1K（普通分辨率），大于此值需要4K渠道

**默认配置**：`min_edge` 模式，阈值 `1024`

### 2. 数据模型

#### 2.1 系统配置

新增配置模块 `setting/image_size_setting/`：

```go
type ImageSizeSetting struct {
    Mode           ImageSizeMode `json:"mode"`             // total_pixels | min_edge | max_edge
    ThresholdValue int           `json:"threshold_value"`  // 阈值（像素数或边长）
}
```

存储位置：
- 使用现有的系统配置存储机制（数据库或配置文件）
- 配置键：`ImageSizeMode` 和 `ImageSizeThreshold`

#### 2.2 渠道配置

扩展 `dto.ChannelSettings`：

```go
type ChannelSettings struct {
    // ... 现有字段
    Support4K bool `json:"support_4k,omitempty"` // 是否支持4K图片生成
}
```

存储在 `model.Channel.Setting` 字段的 JSON 中。

### 3. 核心逻辑

#### 3.1 尺寸判断

```go
// setting/image_size_setting/image_size.go

func IsHighResolution(width, height int) bool {
    setting := GetSetting()
    
    switch setting.Mode {
    case ModeTotalPixels:
        return width * height > setting.ThresholdValue
    case ModeMinEdge:
        minEdge := min(width, height)
        return minEdge > setting.ThresholdValue
    case ModeMaxEdge:
        maxEdge := max(width, height)
        return maxEdge > setting.ThresholdValue
    default:
        return false
    }
}
```

#### 3.2 请求尺寸解析

在 `middleware/distributor.go` 中解析图片尺寸：

```go
if strings.HasPrefix(c.Request.URL.Path, "/v1/images/generations") {
    modelRequest.Model = common.GetStringIfEmpty(modelRequest.Model, "dall-e")
    
    // 解析图片尺寸
    req, err := getModelFromRequest(c)
    if err == nil && req.Size != "" {
        width, height := parseImageSize(req.Size)
        if width > 0 && height > 0 {
            isHighRes := image_size_setting.IsHighResolution(width, height)
            common.SetContextKey(c, constant.ContextKeyRequire4K, isHighRes)
        }
    }
}
```

尺寸字符串格式：`"1024x1024"` → 解析为 `(1024, 1024)`

#### 3.3 渠道选择过滤

在 `service/channel_select.go` 的渠道选择逻辑中添加过滤：

```go
func filterChannelsBy4KSupport(ctx *gin.Context, channels []*model.Channel) []*model.Channel {
    require4K := common.GetContextKeyBool(ctx, constant.ContextKeyRequire4K)
    if !require4K {
        return channels // 不需要4K，所有渠道都可用
    }
    
    // 需要4K，过滤掉不支持的渠道
    filtered := make([]*model.Channel, 0)
    for _, ch := range channels {
        settings := parseChannelSettings(ch.Setting)
        if settings.Support4K {
            filtered = append(filtered, ch)
        }
    }
    return filtered
}
```

### 4. 前端界面

#### 4.1 系统设置页面

位置：`web/classic/src/pages/Setting/Operation/`

新增或扩展系统设置页面，添加：
- 下拉选择框：选择判断模式（总像素数/最小边长/最大边长）
- 数字输入框：设置阈值
- 提示信息：显示常见配置示例

#### 4.2 渠道编辑页面

位置：`web/classic/src/components/table/channels/modals/EditChannelModal.jsx`

在渠道设置区域添加：
- 复选框：`support_4k`
- 标签：`渠道是否支持4K图片生成`
- 提示文本：`勾选后，该渠道可处理高分辨率图片生成请求（根据系统设置的尺寸阈值判断）`

### 5. API 接口

#### 5.1 系统设置 API

```
GET  /api/setting/image-size       - 获取图片尺寸设置
PUT  /api/setting/image-size       - 更新图片尺寸设置
```

**请求示例**：
```json
{
  "mode": "min_edge",
  "threshold_value": 1024
}
```

**响应示例**：
```json
{
  "success": true,
  "data": {
    "mode": "min_edge",
    "threshold_value": 1024
  }
}
```

#### 5.2 渠道 API

现有的渠道增删改查接口无需改动，只需在：
- `POST /api/channel`（新增渠道）
- `PUT /api/channel/:id`（更新渠道）

的请求 body 中支持 `setting.support_4k` 字段。

### 6. 数据流程

```
用户请求 /v1/images/generations
    ↓
解析 size 参数 (如 "2048x2048")
    ↓
调用 IsHighResolution(2048, 2048)
    ↓
根据系统配置判断 (如 min_edge 模式, 2048 > 1024 → true)
    ↓
设置 ContextKeyRequire4K = true
    ↓
渠道选择逻辑获取候选渠道列表
    ↓
过滤：仅保留 support_4k = true 的渠道
    ↓
从过滤后的渠道中选择一个
    ↓
路由请求到选中的渠道
```

## 边界情况处理

1. **未配置系统设置**：使用默认值（`min_edge`, `1024`）
2. **未配置渠道4K支持**：默认视为支持所有尺寸（向后兼容）
3. **请求未包含 size 参数**：不进行4K过滤，所有渠道可用
4. **size 参数格式错误**：忽略过滤，所有渠道可用
5. **所有渠道都不支持4K**：正常返回错误，提示无可用渠道

## 测试计划

### 单元测试

1. `IsHighResolution` 函数在三种模式下的判断逻辑
2. `parseImageSize` 函数解析各种格式的尺寸字符串
3. `filterChannelsBy4KSupport` 函数的过滤逻辑

### 集成测试

1. 配置不同的系统设置，验证渠道选择结果
2. 配置不同的渠道4K支持，验证路由行为
3. 测试边界情况（未配置、格式错误、无可用渠道等）

### 手动测试

1. 在前端配置系统设置，验证保存和读取
2. 在前端配置渠道4K支持，验证保存和读取
3. 发送不同尺寸的图片生成请求，验证路由到正确的渠道

## 实现计划

1. **后端核心逻辑**
   - 创建 `setting/image_size_setting/` 模块
   - 扩展 `dto.ChannelSettings` 添加 `Support4K` 字段
   - 实现尺寸判断逻辑 `IsHighResolution`
   - 实现尺寸解析逻辑 `parseImageSize`
   - 在 `middleware/distributor.go` 中添加尺寸解析
   - 在 `service/channel_select.go` 中添加渠道过滤

2. **后端 API 接口**
   - 实现系统设置的 GET/PUT 接口
   - 添加路由注册

3. **前端系统设置页面**
   - 添加图片尺寸设置表单
   - 实现与后端 API 的交互

4. **前端渠道编辑页面**
   - 在渠道编辑表单中添加 `support_4k` 复选框
   - 更新表单提交逻辑

5. **测试**
   - 编写单元测试
   - 执行集成测试
   - 执行手动测试

6. **文档和国际化**
   - 添加中英文翻译
   - 更新用户文档

## 国际化

需要添加的翻译键：

**中文**：
- `图片生成尺寸设置`
- `1K图片判断模式`
- `用于区分普通分辨率和高分辨率图片生成请求`
- `总像素数`
- `最小边长`
- `最大边长`
- `1K阈值`
- `小于等于此值的图片为1K（普通分辨率），大于此值需要4K渠道`
- `渠道是否支持4K图片生成`
- `勾选后，该渠道可处理高分辨率图片生成请求（根据系统设置的尺寸阈值判断）`

**英文**：
- `Image Generation Size Settings`
- `1K Image Classification Mode`
- `Used to distinguish between normal and high-resolution image generation requests`
- `Total Pixels`
- `Minimum Edge Length`
- `Maximum Edge Length`
- `1K Threshold`
- `Images less than or equal to this value are 1K (normal resolution), greater than this value require 4K channels`
- `Channel supports 4K image generation`
- `When checked, this channel can handle high-resolution image generation requests (determined by system threshold settings)`

## 风险和限制

1. **兼容性风险**：现有渠道未配置 `support_4k` 将默认支持所有尺寸，管理员需手动标记不支持4K的渠道
2. **配置复杂度**：需要管理员理解三种判断模式的区别
3. **尺寸解析**：依赖客户端传递正确的 `size` 参数格式

## 未来扩展

1. **多级分辨率支持**：支持更细粒度的分类（1K/2K/4K/8K）
2. **渠道自动探测**：通过测试请求自动探测渠道支持的最大分辨率
3. **动态阈值调整**：根据渠道负载动态调整分辨率阈值

## 参考

- [OpenAI Image Generation Guide](https://developers.openai.com/api/docs/guides/image-generation)
- OpenAI 2K 定义：总像素 > 2560x1440 (3,686,400)
- OpenAI 4K 示例：3840x2160, 2160x3840
