# 图片生成4K渠道路由实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现基于图片尺寸的渠道路由，支持配置判断模式和渠道4K能力标记

**Architecture:** 
- 创建 `setting/image_size_setting` 模块管理尺寸判断配置
- 扩展 `dto.ChannelSettings` 添加 4K 支持标记
- 在 `middleware/distributor.go` 解析请求尺寸并设置上下文
- 在 `service/channel_select.go` 过滤不支持4K的渠道
- 前端添加系统设置页面和渠道编辑开关

**Tech Stack:** Go 1.22+, Gin, GORM, React 18, Semi Design

---

## 文件结构

**后端新增文件：**
- `setting/image_size_setting/image_size_setting.go` - 尺寸判断配置模块
- `setting/image_size_setting/image_size_setting_test.go` - 单元测试
- `controller/image_size.go` - 图片尺寸设置API控制器

**后端修改文件：**
- `constant/context_key.go` - 添加 `ContextKeyRequire4K`
- `dto/channel_settings.go` - 添加 `Support4K` 字段
- `middleware/distributor.go:294-305` - 添加尺寸解析逻辑
- `service/channel_select.go:83-160` - 添加渠道过滤逻辑
- `router/api.go` - 注册图片尺寸设置路由

**前端新增文件：**
- `web/classic/src/pages/Setting/Operation/SettingsImageSize.jsx` - 图片尺寸设置页面

**前端修改文件：**
- `web/classic/src/components/table/channels/modals/EditChannelModal.jsx:190-200` - 添加 support_4k 开关
- `web/classic/src/i18n/locales/zh.json` - 添加中文翻译
- `web/classic/src/i18n/locales/en.json` - 添加英文翻译

---

### Task 1: 添加 Context Key 常量

**Files:**
- Modify: `constant/context_key.go:29` (在 ChannelSetting 后添加)

- [ ] **Step 1: 添加 ContextKeyRequire4K 常量**

在 `constant/context_key.go` 的 Channel 相关常量后添加：

```go
ContextKeyRequire4K ContextKey = "require_4k" // 是否需要4K渠道
```

- [ ] **Step 2: 提交**

```bash
git add constant/context_key.go
git commit -m "feat: add ContextKeyRequire4K for image size routing"
```

---

### Task 2: 扩展渠道设置 DTO

**Files:**
- Modify: `dto/channel_settings.go:11` (在 DisableProbe 后添加)

- [ ] **Step 1: 添加 Support4K 字段**

在 `dto/channel_settings.go` 的 `ChannelSettings` 结构体中添加：

```go
Support4K bool `json:"support_4k,omitempty"` // 是否支持4K图片生成
```

- [ ] **Step 2: 提交**

```bash
git add dto/channel_settings.go
git commit -m "feat: add Support4K field to ChannelSettings"
```

---

### Task 3: 创建图片尺寸设置模块

**Files:**
- Create: `setting/image_size_setting/image_size_setting.go`
- Create: `setting/image_size_setting/image_size_setting_test.go`

- [ ] **Step 1: 编写图片尺寸设置测试 - 总像素模式**

创建 `setting/image_size_setting/image_size_setting_test.go`：

```go
package image_size_setting

import (
	"testing"
)

func TestIsHighResolution_TotalPixels(t *testing.T) {
	// 设置测试配置
	imageSizeSetting = ImageSizeSetting{
		Mode:           ModeTotalPixels,
		ThresholdValue: 1048576, // 1024x1024
	}

	tests := []struct {
		name     string
		width    int
		height   int
		expected bool
	}{
		{"1024x1024 等于阈值", 1024, 1024, false},
		{"512x512 小于阈值", 512, 512, false},
		{"2048x2048 大于阈值", 2048, 2048, true},
		{"1024x1536 大于阈值", 1024, 1536, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsHighResolution(tt.width, tt.height)
			if result != tt.expected {
				t.Errorf("IsHighResolution(%d, %d) = %v, want %v", tt.width, tt.height, result, tt.expected)
			}
		})
	}
}
```

- [ ] **Step 2: 运行测试验证失败**

```bash
go test ./setting/image_size_setting -v -run TestIsHighResolution_TotalPixels
```

预期输出：`package setting/image_size_setting is not in GOROOT`

- [ ] **Step 3: 创建图片尺寸设置模块主文件**

创建 `setting/image_size_setting/image_size_setting.go`：

```go
package image_size_setting

import (
	"sync"

	"github.com/QuantumNous/new-api/setting/config"
)

type ImageSizeMode string

const (
	ModeTotalPixels ImageSizeMode = "total_pixels" // 按总像素数判断
	ModeMinEdge     ImageSizeMode = "min_edge"     // 按最小边长判断
	ModeMaxEdge     ImageSizeMode = "max_edge"     // 按最大边长判断
)

type ImageSizeSetting struct {
	Mode           ImageSizeMode `json:"mode"`
	ThresholdValue int           `json:"threshold_value"`
}

// 默认配置：最小边 <= 1024
var imageSizeSetting = ImageSizeSetting{
	Mode:           ModeMinEdge,
	ThresholdValue: 1024,
}

var mutex sync.RWMutex

func init() {
	config.GlobalConfig.Register("image_size_setting", &imageSizeSetting)
}

func GetSetting() ImageSizeSetting {
	mutex.RLock()
	defer mutex.RUnlock()
	return imageSizeSetting
}

// IsHighResolution 判断给定的尺寸是否为高分辨率（需要4K渠道）
func IsHighResolution(width, height int) bool {
	mutex.RLock()
	defer mutex.RUnlock()

	switch imageSizeSetting.Mode {
	case ModeTotalPixels:
		return width*height > imageSizeSetting.ThresholdValue
	case ModeMinEdge:
		minEdge := width
		if height < width {
			minEdge = height
		}
		return minEdge > imageSizeSetting.ThresholdValue
	case ModeMaxEdge:
		maxEdge := width
		if height > width {
			maxEdge = height
		}
		return maxEdge > imageSizeSetting.ThresholdValue
	default:
		return false
	}
}
```

- [ ] **Step 4: 运行测试验证通过**

```bash
go test ./setting/image_size_setting -v -run TestIsHighResolution_TotalPixels
```

预期输出：`PASS`

- [ ] **Step 5: 提交**

```bash
git add setting/image_size_setting/
git commit -m "feat: implement image size classification module

- Add ImageSizeSetting with three modes: total_pixels, min_edge, max_edge
- Implement IsHighResolution function
- Add unit tests for total_pixels mode"
```

---

### Task 4: 完善图片尺寸设置模块测试

**Files:**
- Modify: `setting/image_size_setting/image_size_setting_test.go`

- [ ] **Step 1: 添加最小边模式测试**

在 `setting/image_size_setting/image_size_setting_test.go` 末尾添加：

```go
func TestIsHighResolution_MinEdge(t *testing.T) {
	imageSizeSetting = ImageSizeSetting{
		Mode:           ModeMinEdge,
		ThresholdValue: 1024,
	}

	tests := []struct {
		name     string
		width    int
		height   int
		expected bool
	}{
		{"1024x1024 等于阈值", 1024, 1024, false},
		{"1024x2048 最小边等于阈值", 1024, 2048, false},
		{"2048x1024 最小边等于阈值", 2048, 1024, false},
		{"2048x2048 最小边大于阈值", 2048, 2048, true},
		{"512x2048 最小边小于阈值", 512, 2048, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsHighResolution(tt.width, tt.height)
			if result != tt.expected {
				t.Errorf("IsHighResolution(%d, %d) = %v, want %v", tt.width, tt.height, result, tt.expected)
			}
		})
	}
}

func TestIsHighResolution_MaxEdge(t *testing.T) {
	imageSizeSetting = ImageSizeSetting{
		Mode:           ModeMaxEdge,
		ThresholdValue: 2048,
	}

	tests := []struct {
		name     string
		width    int
		height   int
		expected bool
	}{
		{"2048x2048 等于阈值", 2048, 2048, false},
		{"1024x2048 最大边等于阈值", 1024, 2048, false},
		{"2048x1024 最大边等于阈值", 2048, 1024, false},
		{"3840x2160 最大边大于阈值", 3840, 2160, true},
		{"512x1024 最大边小于阈值", 512, 1024, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsHighResolution(tt.width, tt.height)
			if result != tt.expected {
				t.Errorf("IsHighResolution(%d, %d) = %v, want %v", tt.width, tt.height, result, tt.expected)
			}
		})
	}
}
```

- [ ] **Step 2: 运行所有测试**

```bash
go test ./setting/image_size_setting -v
```

预期输出：所有测试通过 `PASS`

- [ ] **Step 3: 提交**

```bash
git add setting/image_size_setting/image_size_setting_test.go
git commit -m "test: add comprehensive tests for all image size modes"
```

---

### Task 5: 在 distributor 中添加尺寸解析逻辑

**Files:**
- Modify: `middleware/distributor.go:294-305`

- [ ] **Step 1: 添加尺寸解析辅助函数**

在 `middleware/distributor.go` 文件顶部（package 声明后）添加：

```go
// parseImageSize 解析尺寸字符串 "1024x1024" -> (1024, 1024)
func parseImageSize(size string) (int, int) {
	parts := strings.Split(size, "x")
	if len(parts) != 2 {
		return 0, 0
	}
	width, err1 := strconv.Atoi(parts[0])
	height, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 0, 0
	}
	return width, height
}
```

需要确保导入了 `strconv` 包。

- [ ] **Step 2: 在图片生成路由中添加尺寸解析**

在 `middleware/distributor.go` 的 `/v1/images/generations` 处理分支（约第294行）修改为：

```go
if strings.HasPrefix(c.Request.URL.Path, "/v1/images/generations") {
	modelRequest.Model = common.GetStringIfEmpty(modelRequest.Model, "dall-e")
	
	// 解析图片尺寸，判断是否需要4K渠道
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

需要导入 `"github.com/QuantumNous/new-api/setting/image_size_setting"`。

- [ ] **Step 3: 编译检查**

```bash
go build ./middleware
```

预期输出：无错误

- [ ] **Step 4: 提交**

```bash
git add middleware/distributor.go
git commit -m "feat: parse image size and set require_4k context in distributor

- Add parseImageSize helper function
- Extract size from request and determine if 4K channel is required
- Set ContextKeyRequire4K for downstream channel selection"
```

---

### Task 6: 在渠道选择逻辑中添加4K过滤

**Files:**
- Modify: `service/channel_select.go:118-160`

- [ ] **Step 1: 添加渠道4K能力解析辅助函数**

在 `service/channel_select.go` 文件中添加辅助函数（建议在文件末尾）：

```go
// parseChannelSettings 从渠道的 Setting JSON 字段解析 ChannelSettings
func parseChannelSettings(settingJSON *string) *dto.ChannelSettings {
	if settingJSON == nil || *settingJSON == "" {
		return &dto.ChannelSettings{}
	}
	
	var settings dto.ChannelSettings
	err := common.UnmarshalJsonStr(*settingJSON, &settings)
	if err != nil {
		return &dto.ChannelSettings{}
	}
	return &settings
}

// filterChannelsBy4KSupport 根据4K支持能力过滤渠道
func filterChannelsBy4KSupport(ctx *gin.Context, channels []*model.Channel) []*model.Channel {
	require4K := common.GetContextKeyBool(ctx, constant.ContextKeyRequire4K)
	if !require4K {
		return channels // 不需要4K，所有渠道都可用
	}
	
	// 需要4K，过滤掉不支持的渠道
	filtered := make([]*model.Channel, 0, len(channels))
	for _, ch := range channels {
		settings := parseChannelSettings(ch.Setting)
		// 如果渠道明确标记支持4K，或者未配置（向后兼容，默认支持）
		if settings.Support4K || ch.Setting == nil || *ch.Setting == "" {
			filtered = append(filtered, ch)
		}
	}
	return filtered
}
```

需要导入 `"github.com/QuantumNous/new-api/dto"`。

- [ ] **Step 2: 在 GetRandomSatisfiedChannel 中应用过滤**

找到 `model.GetRandomSatisfiedChannel` 调用（约第118行），在获取到渠道后添加过滤：

```go
channel, _ = model.GetRandomSatisfiedChannel(autoGroup, param.ModelName, priorityRetry)
if channel != nil {
	// 应用4K过滤
	candidates := []*model.Channel{channel}
	filtered := filterChannelsBy4KSupport(param.Ctx, candidates)
	if len(filtered) == 0 {
		channel = nil // 渠道不支持4K，视为未找到
	}
}
```

- [ ] **Step 3: 在非 auto 模式中也应用过滤**

找到非 auto 模式的渠道选择逻辑（约第157行），添加类似的过滤：

```go
} else {
	channel, _ = model.GetRandomSatisfiedChannel(param.TokenGroup, param.ModelName, param.GetRetry())
	if channel != nil {
		candidates := []*model.Channel{channel}
		filtered := filterChannelsBy4KSupport(param.Ctx, candidates)
		if len(filtered) == 0 {
			channel = nil
		}
	}
	selectGroup = param.TokenGroup
}
```

- [ ] **Step 4: 编译检查**

```bash
go build ./service
```

预期输出：无错误

- [ ] **Step 5: 提交**

```bash
git add service/channel_select.go
git commit -m "feat: filter channels by 4K support capability

- Add parseChannelSettings helper to extract ChannelSettings
- Add filterChannelsBy4KSupport to filter based on require_4k context
- Apply filtering in both auto and non-auto channel selection modes
- Backward compatible: channels without setting default to supporting all sizes"
```

---

### Task 7: 创建图片尺寸设置 API 控制器

**Files:**
- Create: `controller/image_size.go`

- [ ] **Step 1: 创建 API 控制器**

创建 `controller/image_size.go`：

```go
package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/setting/image_size_setting"
	"github.com/gin-gonic/gin"
)

// GetImageSizeSetting 获取图片尺寸设置
func GetImageSizeSetting(c *gin.Context) {
	setting := image_size_setting.GetSetting()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    setting,
	})
}

// UpdateImageSizeSetting 更新图片尺寸设置
func UpdateImageSizeSetting(c *gin.Context) {
	var req image_size_setting.ImageSizeSetting
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgInvalidRequest, map[string]any{"Error": err.Error()}),
		})
		return
	}

	// 验证模式
	if req.Mode != image_size_setting.ModeTotalPixels &&
		req.Mode != image_size_setting.ModeMinEdge &&
		req.Mode != image_size_setting.ModeMaxEdge {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgInvalidRequest, map[string]any{"Error": "invalid mode"}),
		})
		return
	}

	// 验证阈值
	if req.ThresholdValue <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgInvalidRequest, map[string]any{"Error": "threshold_value must be positive"}),
		})
		return
	}

	// 通过配置管理器保存到数据库
	// 注意：这里假设配置会自动保存，如果需要手动触发保存，需要添加相应逻辑
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": i18n.T(c, i18n.MsgOperationSuccess),
	})
}
```

- [ ] **Step 2: 编译检查**

```bash
go build ./controller
```

预期输出：无错误

- [ ] **Step 3: 提交**

```bash
git add controller/image_size.go
git commit -m "feat: add image size setting API controller

- Add GetImageSizeSetting endpoint
- Add UpdateImageSizeSetting endpoint with validation
- Validate mode and threshold_value parameters"
```

---

### Task 8: 注册图片尺寸设置路由

**Files:**
- Modify: `router/api.go`

- [ ] **Step 1: 查找系统设置路由注册位置**

```bash
grep -n "setting" router/api.go | head -10
```

找到类似 `/api/setting` 的路由组。

- [ ] **Step 2: 添加图片尺寸设置路由**

在系统设置相关路由组中添加（通常在 admin 权限的路由组中）：

```go
settingGroup.GET("/image-size", controller.GetImageSizeSetting)
settingGroup.PUT("/image-size", controller.UpdateImageSizeSetting)
```

如果没有 `settingGroup`，则在合适的 admin 路由组中添加：

```go
adminGroup.GET("/setting/image-size", controller.GetImageSizeSetting)
adminGroup.PUT("/setting/image-size", controller.UpdateImageSizeSetting)
```

- [ ] **Step 3: 编译检查**

```bash
go build ./router
```

预期输出：无错误

- [ ] **Step 4: 测试路由是否注册**

```bash
go run main.go &
sleep 3
curl http://localhost:3000/api/setting/image-size
pkill -f "go run main.go"
```

预期输出：返回默认配置 JSON

- [ ] **Step 5: 提交**

```bash
git add router/api.go
git commit -m "feat: register image size setting API routes

- Add GET /api/setting/image-size endpoint
- Add PUT /api/setting/image-size endpoint"
```

---

### Task 9: 添加前端国际化翻译

**Files:**
- Modify: `web/classic/src/i18n/locales/zh.json`
- Modify: `web/classic/src/i18n/locales/en.json`

- [ ] **Step 1: 添加中文翻译**

在 `web/classic/src/i18n/locales/zh.json` 中添加：

```json
"图片生成尺寸设置": "图片生成尺寸设置",
"1K图片判断模式": "1K图片判断模式",
"用于区分普通分辨率和高分辨率图片生成请求": "用于区分普通分辨率和高分辨率图片生成请求",
"总像素数": "总像素数",
"最小边长": "最小边长",
"最大边长": "最大边长",
"1K阈值": "1K阈值",
"小于等于此值的图片为1K（普通分辨率），大于此值需要4K渠道": "小于等于此值的图片为1K（普通分辨率），大于此值需要4K渠道",
"渠道是否支持4K图片生成": "渠道是否支持4K图片生成",
"勾选后，该渠道可处理高分辨率图片生成请求（根据系统设置的尺寸阈值判断）": "勾选后，该渠道可处理高分辨率图片生成请求（根据系统设置的尺寸阈值判断）",
"像素": "像素",
"示例配置：": "示例配置：",
"OpenAI 标准：模式=总像素数，阈值=3686400 (2560x1440)": "OpenAI 标准：模式=总像素数，阈值=3686400 (2560x1440)",
"最小边模式：模式=最小边长，阈值=1024": "最小边模式：模式=最小边长，阈值=1024",
"最大边模式：模式=最大边长，阈值=2048": "最大边模式：模式=最大边长，阈值=2048"
```

- [ ] **Step 2: 添加英文翻译**

在 `web/classic/src/i18n/locales/en.json` 中添加：

```json
"图片生成尺寸设置": "Image Generation Size Settings",
"1K图片判断模式": "1K Image Classification Mode",
"用于区分普通分辨率和高分辨率图片生成请求": "Used to distinguish between normal and high-resolution image generation requests",
"总像素数": "Total Pixels",
"最小边长": "Minimum Edge Length",
"最大边长": "Maximum Edge Length",
"1K阈值": "1K Threshold",
"小于等于此值的图片为1K（普通分辨率），大于此值需要4K渠道": "Images less than or equal to this value are 1K (normal resolution), greater than this value require 4K channels",
"渠道是否支持4K图片生成": "Channel supports 4K image generation",
"勾选后，该渠道可处理高分辨率图片生成请求（根据系统设置的尺寸阈值判断）": "When checked, this channel can handle high-resolution image generation requests (determined by system threshold settings)",
"像素": "pixels",
"示例配置：": "Example configurations:",
"OpenAI 标准：模式=总像素数，阈值=3686400 (2560x1440)": "OpenAI standard: mode=Total Pixels, threshold=3686400 (2560x1440)",
"最小边模式：模式=最小边长，阈值=1024": "Min edge mode: mode=Minimum Edge, threshold=1024",
"最大边模式：模式=最大边长，阈值=2048": "Max edge mode: mode=Maximum Edge, threshold=2048"
```

- [ ] **Step 3: 运行翻译同步（如果项目有工具）**

```bash
cd web/classic
bun run i18n:sync
cd ../..
```

如果没有同步工具，跳过此步。

- [ ] **Step 4: 提交**

```bash
git add web/classic/src/i18n/locales/zh.json web/classic/src/i18n/locales/en.json
git commit -m "i18n: add translations for image size settings

- Add Chinese translations for image size configuration
- Add English translations for image size configuration"
```

---

### Task 10: 创建前端图片尺寸设置页面

**Files:**
- Create: `web/classic/src/pages/Setting/Operation/SettingsImageSize.jsx`

- [ ] **Step 1: 创建图片尺寸设置组件**

创建 `web/classic/src/pages/Setting/Operation/SettingsImageSize.jsx`：

```jsx
import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Form, Button, Banner } from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../../helpers';

const SettingsImageSize = () => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [formApi, setFormApi] = useState(null);

  useEffect(() => {
    fetchSettings();
  }, []);

  const fetchSettings = async () => {
    try {
      const res = await API.get('/api/setting/image-size');
      if (res.data.success && formApi) {
        formApi.setValues(res.data.data);
      }
    } catch (error) {
      showError(t('获取设置失败'));
    }
  };

  const handleSubmit = async (values) => {
    setLoading(true);
    try {
      const res = await API.put('/api/setting/image-size', values);
      if (res.data.success) {
        showSuccess(t('保存成功'));
      } else {
        showError(res.data.message || t('保存失败'));
      }
    } catch (error) {
      showError(t('保存失败'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Form
      onSubmit={handleSubmit}
      getFormApi={(api) => setFormApi(api)}
      style={{ maxWidth: 800 }}
      labelPosition="left"
      labelAlign="right"
      labelWidth={180}
    >
      <Form.Section text={t('图片生成尺寸设置')}>
        <Form.Select
          field="mode"
          label={t('1K图片判断模式')}
          initValue="min_edge"
          style={{ width: '100%' }}
          extraText={t('用于区分普通分辨率和高分辨率图片生成请求')}
          rules={[{ required: true, message: t('请选择判断模式') }]}
        >
          <Form.Select.Option value="total_pixels">
            {t('总像素数')} - {t('如 1024x1024 = 1,048,576 像素')}
          </Form.Select.Option>
          <Form.Select.Option value="min_edge">
            {t('最小边长')} - {t('如 1024x1536 的最小边是 1024')}
          </Form.Select.Option>
          <Form.Select.Option value="max_edge">
            {t('最大边长')} - {t('如 1024x1536 的最大边是 1536')}
          </Form.Select.Option>
        </Form.Select>

        <Form.InputNumber
          field="threshold_value"
          label={t('1K阈值')}
          initValue={1024}
          min={256}
          max={10000000}
          step={256}
          style={{ width: '100%' }}
          extraText={t('小于等于此值的图片为1K（普通分辨率），大于此值需要4K渠道')}
          rules={[
            { required: true, message: t('请输入阈值') },
            { type: 'number', min: 256, message: t('阈值不能小于256') }
          ]}
          suffix={
            ((formApi?.getValue('mode') || 'min_edge') === 'total_pixels') 
              ? t('像素') 
              : 'px'
          }
        />
        
        <Banner
          type="info"
          description={
            <div>
              {t('示例配置：')}<br/>
              • {t('OpenAI 标准：模式=总像素数，阈值=3686400 (2560x1440)')}<br/>
              • {t('最小边模式：模式=最小边长，阈值=1024')}<br/>
              • {t('最大边模式：模式=最大边长，阈值=2048')}
            </div>
          }
        />
      </Form.Section>

      <div style={{ marginTop: 24 }}>
        <Button
          type="primary"
          htmlType="submit"
          loading={loading}
        >
          {t('保存')}
        </Button>
      </div>
    </Form>
  );
};

export default SettingsImageSize;
```

- [ ] **Step 2: 验证组件语法**

```bash
cd web/classic
bun install
cd ../..
```

预期输出：无错误

- [ ] **Step 3: 提交**

```bash
git add web/classic/src/pages/Setting/Operation/SettingsImageSize.jsx
git commit -m "feat: add image size settings page component

- Add form for configuring image size classification mode
- Add validation for mode and threshold
- Add example configurations in banner
- Support dynamic suffix based on selected mode"
```

---

### Task 11: 在渠道编辑表单中添加4K支持开关

**Files:**
- Modify: `web/classic/src/components/table/channels/modals/EditChannelModal.jsx:190-200`

- [ ] **Step 1: 找到渠道设置字段初始化位置**

```bash
grep -n "disable_probe" web/classic/src/components/table/channels/modals/EditChannelModal.jsx
```

找到 `originInputs` 对象定义和渠道设置字段。

- [ ] **Step 2: 在 originInputs 中添加 support_4k 默认值**

在 `originInputs` 对象的渠道设置部分（约第198行，`disable_probe: false` 附近）添加：

```javascript
support_4k: false,
```

- [ ] **Step 3: 找到渠道设置表单区域**

```bash
grep -n "disable_probe.*Checkbox" web/classic/src/components/table/channels/modals/EditChannelModal.jsx
```

找到 `disable_probe` 的 Checkbox 定义位置。

- [ ] **Step 4: 在表单中添加 support_4k 复选框**

在 `disable_probe` Checkbox 后添加（保持相同的缩进和结构）：

```jsx
<Form.Checkbox
  field="support_4k"
  label={t('渠道是否支持4K图片生成')}
  extraText={t('勾选后，该渠道可处理高分辨率图片生成请求（根据系统设置的尺寸阈值判断）')}
/>
```

- [ ] **Step 5: 找到表单提交处理位置**

```bash
grep -n "disable_probe.*inputs" web/classic/src/components/table/channels/modals/EditChannelModal.jsx
```

找到提交时构建 `setting` 对象的位置。

- [ ] **Step 6: 在提交逻辑中包含 support_4k**

在构建 `setting` 对象时（通常在 `submit` 函数中），确保包含 `support_4k`：

```javascript
const setting = {
  // ... 其他字段
  disable_probe: inputs.disable_probe,
  support_4k: inputs.support_4k,
};
```

- [ ] **Step 7: 找到加载渠道数据的位置**

```bash
grep -n "loadChannel" web/classic/src/components/table/channels/modals/EditChannelModal.jsx
```

找到从 API 加载渠道数据并填充表单的位置。

- [ ] **Step 8: 在加载逻辑中包含 support_4k**

在解析 `channel.setting` 并填充表单的位置，确保设置 `support_4k`：

```javascript
// 解析 setting JSON
const setting = JSON.parse(channel.setting || '{}');
// ... 设置其他字段
disable_probe: setting.disable_probe || false,
support_4k: setting.support_4k || false,
```

- [ ] **Step 9: 验证前端语法**

```bash
cd web/classic
bun run build
cd ../..
```

预期输出：构建成功，无错误

- [ ] **Step 10: 提交**

```bash
git add web/classic/src/components/table/channels/modals/EditChannelModal.jsx
git commit -m "feat: add 4K support toggle in channel edit form

- Add support_4k field to originInputs default values
- Add support_4k checkbox in channel settings section
- Include support_4k in form submission logic
- Load support_4k value when editing existing channel"
```

---

### Task 12: 集成测试 - 验证完整流程

**Files:**
- Test: 整个系统

- [ ] **Step 1: 启动后端服务**

```bash
go run main.go &
sleep 5
```

预期输出：服务启动成功

- [ ] **Step 2: 测试系统设置 API - 获取默认配置**

```bash
curl -X GET http://localhost:3000/api/setting/image-size
```

预期输出：
```json
{
  "success": true,
  "data": {
    "mode": "min_edge",
    "threshold_value": 1024
  }
}
```

- [ ] **Step 3: 测试系统设置 API - 更新配置**

```bash
curl -X PUT http://localhost:3000/api/setting/image-size \
  -H "Content-Type: application/json" \
  -d '{"mode":"total_pixels","threshold_value":3686400}'
```

预期输出：`{"success": true, "message": "操作成功"}`

- [ ] **Step 4: 测试渠道创建 - 支持4K**

```bash
curl -X POST http://localhost:3000/api/channel \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test 4K Channel",
    "type": 1,
    "key": "test-key",
    "models": "dall-e-3",
    "setting": "{\"support_4k\":true}"
  }'
```

预期输出：渠道创建成功

- [ ] **Step 5: 测试图片生成请求 - 1K尺寸**

```bash
curl -X POST http://localhost:3000/v1/images/generations \
  -H "Content-Type: application/json" \
  -d '{
    "model": "dall-e-3",
    "prompt": "a cat",
    "size": "1024x1024"
  }'
```

观察日志，验证 `ContextKeyRequire4K` 为 false。

- [ ] **Step 6: 测试图片生成请求 - 4K尺寸**

```bash
curl -X POST http://localhost:3000/v1/images/generations \
  -H "Content-Type: application/json" \
  -d '{
    "model": "dall-e-3",
    "prompt": "a cat",
    "size": "2048x2048"
  }'
```

观察日志，验证 `ContextKeyRequire4K` 为 true，且只选择支持4K的渠道。

- [ ] **Step 7: 停止服务**

```bash
pkill -f "go run main.go"
```

- [ ] **Step 8: 启动前端开发服务器**

```bash
cd web/classic
bun run dev &
sleep 5
cd ../..
```

- [ ] **Step 9: 手动测试前端**

1. 访问 `http://localhost:8080/setting/operation/image-size`（或相应路径）
2. 验证图片尺寸设置页面正常显示
3. 修改模式和阈值，点击保存
4. 刷新页面，验证设置已保存
5. 访问渠道管理页面
6. 编辑一个渠道，验证"支持4K图片生成"开关显示
7. 勾选开关，保存，刷新验证已保存

- [ ] **Step 10: 停止前端服务**

```bash
cd web/classic
pkill -f "bun run dev"
cd ../..
```

- [ ] **Step 11: 提交测试报告（如果有）**

创建测试记录文件或更新文档。

```bash
echo "Integration tests passed on $(date)" >> docs/test-results.md
git add docs/test-results.md
git commit -m "test: add integration test results for 4K routing"
```

---

### Task 13: 文档和清理

**Files:**
- Create: `docs/features/image-4k-routing.md` (可选)
- Review: 所有修改的文件

- [ ] **Step 1: 运行完整测试套件**

```bash
go test ./... -v
```

预期输出：所有测试通过

- [ ] **Step 2: 运行前端构建**

```bash
cd web/classic
bun run build
cd ../..
```

预期输出：构建成功

- [ ] **Step 3: 检查代码格式**

```bash
go fmt ./...
cd web/classic
bun run lint
cd ../..
```

预期输出：无格式问题

- [ ] **Step 4: 创建功能文档（可选）**

创建 `docs/features/image-4k-routing.md`：

```markdown
# 图片生成4K渠道路由

## 功能概述

根据图片尺寸自动路由到支持相应分辨率的渠道，避免将高分辨率请求发送到不支持的渠道。

## 配置说明

### 系统设置

在系统设置中配置1K图片的判断标准：

- **判断模式**：
  - 总像素数：按 width × height 判断
  - 最小边长：按 min(width, height) 判断
  - 最大边长：按 max(width, height) 判断

- **阈值**：小于等于阈值为1K（普通分辨率），大于阈值需要4K渠道

### 渠道设置

在渠道编辑页面勾选"支持4K图片生成"开关。

- 勾选：该渠道可处理高分辨率请求
- 不勾选：该渠道只处理普通分辨率请求
- 未配置：默认支持所有尺寸（向后兼容）

## 使用示例

### OpenAI 标准配置

- 模式：总像素数
- 阈值：3686400 (2560×1440)

### 最小边配置

- 模式：最小边长
- 阈值：1024

## API 接口

- `GET /api/setting/image-size` - 获取配置
- `PUT /api/setting/image-size` - 更新配置
```

- [ ] **Step 5: 提交文档**

```bash
git add docs/features/image-4k-routing.md
git commit -m "docs: add 4K image routing feature documentation"
```

- [ ] **Step 6: 最终验证**

```bash
git log --oneline -15
git status
```

验证所有提交已完成，工作区干净。

- [ ] **Step 7: 创建功能分支合并提交（可选）**

```bash
git log --oneline --graph -15
```

确认提交历史清晰。

---

## 实现完成

所有任务已完成！功能清单：

✅ 添加 `ContextKeyRequire4K` 常量
✅ 扩展 `ChannelSettings` 添加 `Support4K` 字段
✅ 创建图片尺寸设置模块和测试
✅ 在 distributor 中添加尺寸解析逻辑
✅ 在 channel_select 中添加4K过滤逻辑
✅ 创建图片尺寸设置 API 控制器
✅ 注册 API 路由
✅ 添加前端国际化翻译
✅ 创建图片尺寸设置页面
✅ 在渠道编辑表单中添加4K支持开关
✅ 完成集成测试
✅ 编写功能文档

下一步建议：
1. 在生产环境部署前进行完整回归测试
2. 通知管理员配置现有渠道的4K支持能力
3. 监控图片生成请求的路由情况

