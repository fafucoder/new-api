package ratio_setting

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/types"
)

var defaultGroupRatio = map[string]float64{
	"default": 1,
	"vip":     1,
	"svip":    1,
}

var groupRatioMap = types.NewRWMap[string, float64]()

var defaultGroupGroupRatio = map[string]map[string]float64{
	"vip": {
		"edit_this": 0.9,
	},
}

var groupGroupRatioMap = types.NewRWMap[string, map[string]float64]()

var defaultGroupSpecialUsableGroup = map[string]map[string]string{
	"vip": {
		"append_1":   "vip_special_group_1",
		"-:remove_1": "vip_removed_group_1",
	},
}

// defaultGroupModelPrice 分组特殊模型定价：分组 -> 模型名 -> 绝对价格（按次/按量的固定价格）
// 命中后使用该绝对价格作为模型价格（usePrice=true），随后仍会乘以分组倍率。
var defaultGroupModelPrice = map[string]map[string]float64{}

var groupModelPriceMap = types.NewRWMap[string, map[string]float64]()

// defaultGroupModelImageOnly 分组模型「仅允许生图接口」白名单：分组 -> 模型名 -> true 表示该模型为生图模型
// 语义为白名单：仅显式配置为 true 的「分组+模型」组合被视为生图模型，只允许走 /v1/images/generations 与 /v1/images/edits，其余接口一律拒绝。
// 典型场景：生图模型（如 gpt-image-2）不应被 /v1/chat/completions 或 /v1/responses 调用，否则按次计费异常。
var defaultGroupModelImageOnly = map[string]map[string]bool{}

var groupModelImageOnlyMap = types.NewRWMap[string, map[string]bool]()

var groupTypeMap = types.NewRWMap[string, string]()

type GroupRatioSetting struct {
	GroupRatio              *types.RWMap[string, float64]            `json:"group_ratio"`
	GroupGroupRatio         *types.RWMap[string, map[string]float64] `json:"group_group_ratio"`
	GroupSpecialUsableGroup *types.RWMap[string, map[string]string]  `json:"group_special_usable_group"`
	GroupModelPrice         *types.RWMap[string, map[string]float64] `json:"group_model_price"`
	GroupModelImageOnly     *types.RWMap[string, map[string]bool]    `json:"group_model_image_only"`
	GroupType               *types.RWMap[string, string]             `json:"group_type"`
}

var groupRatioSetting GroupRatioSetting

func init() {
	groupSpecialUsableGroup := types.NewRWMap[string, map[string]string]()
	groupSpecialUsableGroup.AddAll(defaultGroupSpecialUsableGroup)

	groupRatioMap.AddAll(defaultGroupRatio)
	groupGroupRatioMap.AddAll(defaultGroupGroupRatio)
	groupModelPriceMap.AddAll(defaultGroupModelPrice)
	groupModelImageOnlyMap.AddAll(defaultGroupModelImageOnly)

	groupRatioSetting = GroupRatioSetting{
		GroupSpecialUsableGroup: groupSpecialUsableGroup,
		GroupRatio:              groupRatioMap,
		GroupGroupRatio:         groupGroupRatioMap,
		GroupModelPrice:         groupModelPriceMap,
		GroupModelImageOnly:     groupModelImageOnlyMap,
		GroupType:               groupTypeMap,
	}

	config.GlobalConfig.Register("group_ratio_setting", &groupRatioSetting)
}

func GetGroupRatioSetting() *GroupRatioSetting {
	if groupRatioSetting.GroupSpecialUsableGroup == nil {
		groupRatioSetting.GroupSpecialUsableGroup = types.NewRWMap[string, map[string]string]()
		groupRatioSetting.GroupSpecialUsableGroup.AddAll(defaultGroupSpecialUsableGroup)
	}
	if groupRatioSetting.GroupModelPrice == nil {
		groupRatioSetting.GroupModelPrice = groupModelPriceMap
	}
	if groupRatioSetting.GroupModelImageOnly == nil {
		groupRatioSetting.GroupModelImageOnly = groupModelImageOnlyMap
	}
	if groupRatioSetting.GroupType == nil {
		groupRatioSetting.GroupType = groupTypeMap
	}
	return &groupRatioSetting
}

func GetGroupRatioCopy() map[string]float64 {
	return groupRatioMap.ReadAll()
}

func GetGroupTypeCopy() map[string]string {
	return groupTypeMap.ReadAll()
}

func ContainsGroupRatio(name string) bool {
	_, ok := groupRatioMap.Get(name)
	return ok
}

func GroupRatio2JSONString() string {
	return groupRatioMap.MarshalJSONString()
}

func UpdateGroupRatioByJSONString(jsonStr string) error {
	return types.LoadFromJsonString(groupRatioMap, jsonStr)
}

func GetGroupRatio(name string) float64 {
	ratio, ok := groupRatioMap.Get(name)
	if !ok {
		common.SysLog("group ratio not found: " + name)
		return 1
	}
	return ratio
}

func GetGroupGroupRatio(userGroup, usingGroup string) (float64, bool) {
	gp, ok := groupGroupRatioMap.Get(userGroup)
	if !ok {
		return -1, false
	}
	ratio, ok := gp[usingGroup]
	if !ok {
		return -1, false
	}
	return ratio, true
}

func GroupGroupRatio2JSONString() string {
	return groupGroupRatioMap.MarshalJSONString()
}

func UpdateGroupGroupRatioByJSONString(jsonStr string) error {
	return types.LoadFromJsonString(groupGroupRatioMap, jsonStr)
}

func CheckGroupRatio(jsonStr string) error {
	checkGroupRatio := make(map[string]float64)
	err := json.Unmarshal([]byte(jsonStr), &checkGroupRatio)
	if err != nil {
		return err
	}
	for name, ratio := range checkGroupRatio {
		if ratio < 0 {
			return errors.New("group ratio must be not less than 0: " + name)
		}
	}
	return nil
}

// GroupModelPrice2JSONString 序列化分组特殊模型定价配置
func GroupModelPrice2JSONString() string {
	return groupModelPriceMap.MarshalJSONString()
}

// UpdateGroupModelPriceByJSONString 从 JSON 字符串更新分组特殊模型定价配置
func UpdateGroupModelPriceByJSONString(jsonStr string) error {
	if strings.TrimSpace(jsonStr) == "" {
		jsonStr = "{}"
	}
	return types.LoadFromJsonString(groupModelPriceMap, jsonStr)
}

// GetGroupModelPrice 返回指定分组下指定模型的特殊绝对价格；未配置返回 -1, false
func GetGroupModelPrice(group, modelName string) (float64, bool) {
	gp, ok := groupModelPriceMap.Get(group)
	if !ok {
		return -1, false
	}
	price, ok := gp[modelName]
	if !ok {
		return -1, false
	}
	return price, true
}

// CheckGroupModelPrice 校验分组特殊模型定价 JSON 合法性
func CheckGroupModelPrice(jsonStr string) error {
	if strings.TrimSpace(jsonStr) == "" {
		return nil
	}
	checkGroupModelPrice := make(map[string]map[string]float64)
	err := json.Unmarshal([]byte(jsonStr), &checkGroupModelPrice)
	if err != nil {
		return err
	}
	for group, models := range checkGroupModelPrice {
		for modelName, price := range models {
			if price < 0 {
				return errors.New("group model price must be not less than 0: " + group + "." + modelName)
			}
		}
	}
	return nil
}

// GroupModelImageOnly2JSONString 序列化分组模型「仅允许生图接口」白名单配置
func GroupModelImageOnly2JSONString() string {
	return groupModelImageOnlyMap.MarshalJSONString()
}

// UpdateGroupModelImageOnlyByJSONString 从 JSON 字符串更新分组模型「仅允许生图接口」白名单配置
func UpdateGroupModelImageOnlyByJSONString(jsonStr string) error {
	if strings.TrimSpace(jsonStr) == "" {
		jsonStr = "{}"
	}
	return types.LoadFromJsonString(groupModelImageOnlyMap, jsonStr)
}

// CheckGroupModelImageOnly 校验分组模型「仅允许生图接口」白名单 JSON 合法性
func CheckGroupModelImageOnly(jsonStr string) error {
	if strings.TrimSpace(jsonStr) == "" {
		return nil
	}
	checkGroupModelImageOnly := make(map[string]map[string]bool)
	return json.Unmarshal([]byte(jsonStr), &checkGroupModelImageOnly)
}

// IsGroupModelImageOnly 返回指定分组下指定模型是否为生图模型（仅允许生图接口）
// 语义为白名单：仅当显式配置为 true 时返回 true。
func IsGroupModelImageOnly(group, modelName string) bool {
	gp, ok := groupModelImageOnlyMap.Get(group)
	if !ok {
		return false
	}
	disabled, ok := gp[modelName]
	if !ok {
		return false
	}
	return disabled
}
