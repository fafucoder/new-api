package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/google/uuid"
)

const assetLibraryResponseLimit = 8 << 20

// assetLibraryUploadDir stores client-uploaded files so they can be exposed as
// public URLs. A volcengine-style CreateAsset only accepts a public URL (no
// file/base64 upload), so we host the file locally under ./upload/asset (served
// by router.Static("/upload", ...)) and hand the upstream that URL. OpenAI-style
// upstreams instead read the stored file bytes and POST them to /v1/files.
const (
	assetLibraryUploadDir = "./upload/asset"
	assetLibraryURLPrefix = "/upload/asset/"
)

// Volcengine action defaults, applied when an upstream leaves a field blank.
const (
	assetVolcDefaultVersion           = "2024-01-01"
	assetVolcDefaultCreateGroupAction = "CreateAssetGroup"
	assetVolcDefaultUpdateGroupAction = "UpdateAssetGroup"
	assetVolcDefaultDeleteGroupAction = "DeleteAssetGroup"
	assetVolcDefaultCreateAssetAction = "CreateAsset"
	assetVolcDefaultGetAssetAction    = "GetAsset"
	assetVolcDefaultUpdateAssetAction = "UpdateAsset"
	assetVolcDefaultDeleteAssetAction = "DeleteAsset"
)

const assetOpenAIDefaultPurpose = "user_data"

// AssetLibraryUpstreamBrief is the minimal upstream descriptor exposed to
// ordinary users (for display), without secrets.
type AssetLibraryUpstreamBrief struct {
	Id     int64  `json:"id"`
	Name   string `json:"name"`
	Format string `json:"format"`
}

// AssetLibraryOperationResult reports the outcome of one upstream during a
// multi-upstream fan-out. Internal upstream details are omitted to avoid
// leaking infrastructure information to end users.
type AssetLibraryOperationResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// AssetLibraryGroupResponse is the public-facing representation of a group,
// free of upstream identifiers and mapping details.
type AssetLibraryGroupResponse struct {
	Id          int64                        `json:"id"`
	DisplayName string                       `json:"display_name"`
	Description string                       `json:"description"`
	GroupType   string                       `json:"group_type"`
	CoverURL    string                       `json:"cover_url"`
	CreatedTime int64                        `json:"created_time"`
	UpdatedTime int64                        `json:"updated_time"`
	Status      string                       `json:"status"`
	Assets      []AssetLibraryAssetResponse  `json:"assets"`
}

// AssetLibraryAssetResponse is the public-facing representation of an asset.
type AssetLibraryAssetResponse struct {
	Id          int64  `json:"id"`
	GroupId     int64  `json:"group_id"`
	Name        string `json:"name"`
	AssetType   string `json:"asset_type"`
	SourceURL   string `json:"source_url"`
	FileSize    int64  `json:"file_size"`
	MimeType    string `json:"mime_type"`
	CreatedTime int64  `json:"created_time"`
	UpdatedTime int64  `json:"updated_time"`
	Status      string `json:"status"`
	AssetURL    string `json:"asset_url"`
	AssetId     string `json:"asset_id"`
}

// ---- volcengine response envelope ----

type volcResponseMetadata struct {
	RequestId string     `json:"RequestId"`
	Action    string     `json:"Action"`
	Version   string     `json:"Version"`
	Service   string     `json:"Service"`
	Region    string     `json:"Region"`
	Error     *volcError `json:"Error"`
}

type volcError struct {
	Code    string `json:"Code"`
	Message string `json:"Message"`
}

type volcEnvelope struct {
	ResponseMetadata volcResponseMetadata `json:"ResponseMetadata"`
	Result           json.RawMessage      `json:"Result"`
}

type volcIdResult struct {
	Id string `json:"Id"`
}

type volcGetAssetResult struct {
	Id           string `json:"Id"`
	Name         string `json:"Name"`
	AssetType    string `json:"AssetType"`
	Status       string `json:"Status"`
	URL          string `json:"URL"`
	ErrorCode    string `json:"ErrorCode"`
	ErrorMessage string `json:"ErrorMessage"`
}

// ---- openai Files API response ----

type openAIFileObject struct {
	Id       string `json:"id"`
	Object   string `json:"object"`
	Bytes    int64  `json:"bytes"`
	Filename string `json:"filename"`
	Purpose  string `json:"purpose"`
	Status   string `json:"status"`
}

type openAIErrorEnvelope struct {
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// upstreamAssetResult is the normalized outcome of pushing/querying one asset on
// one upstream, regardless of format.
type upstreamAssetResult struct {
	UpstreamAssetId string
	AssetURL        string
	Status          string
	ErrorCode       string
	ErrorMessage    string
}

// configuredAssetLibraryTargets returns all enabled upstreams.
func configuredAssetLibraryTargets() ([]model.AssetLibraryUpstream, error) {
	return model.ListEnabledAssetLibraryUpstreams()
}

// ListAssetLibraryUpstreamsForUser returns enabled upstreams as briefs for
// display in the asset management page.
func ListAssetLibraryUpstreamsForUser() ([]AssetLibraryUpstreamBrief, error) {
	upstreams, err := configuredAssetLibraryTargets()
	if err != nil {
		return nil, err
	}
	briefs := make([]AssetLibraryUpstreamBrief, 0, len(upstreams))
	for i := range upstreams {
		briefs = append(briefs, AssetLibraryUpstreamBrief{
			Id: upstreams[i].Id, Name: upstreams[i].Name, Format: upstreams[i].Format,
		})
	}
	return briefs, nil
}

func ListAssetLibraryGroups(userId int) ([]AssetLibraryGroupResponse, error) {
	groups, err := model.ListAssetLibraryGroups(userId)
	if err != nil {
		return nil, err
	}
	resp := make([]AssetLibraryGroupResponse, 0, len(groups))
	for i := range groups {
		resp = append(resp, toGroupResponse(&groups[i]))
	}
	return resp, nil
}

func GetAssetLibraryGroup(userId int, groupId int64) (*AssetLibraryGroupResponse, error) {
	group, err := model.GetAssetLibraryGroup(userId, groupId)
	if err != nil {
		return nil, err
	}
	resp := toGroupResponse(group)
	return &resp, nil
}

// CreateAssetLibraryGroup creates an empty asset group locally and, for every
// volcengine upstream, an upstream asset group. OpenAI upstreams have no group
// concept, so only a local mapping placeholder is recorded.
func CreateAssetLibraryGroup(ctx context.Context, userId int, displayName string, groupType string, description string) (*AssetLibraryGroupResponse, []AssetLibraryOperationResult, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return nil, nil, errors.New("display name is required")
	}
	if len([]rune(displayName)) > 64 {
		return nil, nil, errors.New("display name must not exceed 64 characters")
	}
	description = strings.TrimSpace(description)
	if len([]rune(description)) > 300 {
		return nil, nil, errors.New("description must not exceed 300 characters")
	}
	if groupType == "" {
		groupType = "AIGC"
	}

	upstreams, err := configuredAssetLibraryTargets()
	if err != nil {
		return nil, nil, err
	}
	if len(upstreams) == 0 {
		return nil, nil, errors.New("no asset library upstream is configured")
	}

	group := &model.AssetLibraryGroup{
		UserId:      userId,
		DisplayName: displayName,
		Description: description,
		GroupType:   groupType,
		Assets:      []model.AssetLibraryAsset{},
	}
	if err := model.CreateAssetLibraryGroup(group); err != nil {
		return nil, nil, err
	}

	results := make([]AssetLibraryOperationResult, 0, len(upstreams))
	for i := range upstreams {
		upstream := &upstreams[i]
		mapping := &model.AssetLibraryGroupUpstream{
			GroupId: group.Id, UserId: userId, UpstreamId: upstream.Id, Status: "Active",
		}
		upstreamGroupId, groupErr := createGroupOnTarget(ctx, upstream, displayName, groupType)
		result := AssetLibraryOperationResult{Success: groupErr == nil}
		if groupErr != nil {
			mapping.Status = "Failed"
			mapping.ErrorMessage = groupErr.Error()
			result.Message = groupErr.Error()
		} else {
			mapping.UpstreamGroupId = upstreamGroupId
		}
		if err := model.SaveAssetLibraryGroupUpstream(mapping); err != nil {
			return nil, results, err
		}
		results = append(results, result)
	}

	created, err := GetAssetLibraryGroup(userId, group.Id)
	return created, results, err
}

// AppendAssetLibraryFiles adds new assets to an existing group from uploaded
// files.
func AppendAssetLibraryFiles(ctx context.Context, userId int, groupId int64, files []*multipart.FileHeader) (*AssetLibraryGroupResponse, []AssetLibraryOperationResult, error) {
	if len(files) == 0 {
		return nil, nil, errors.New("at least one file is required")
	}
	if len(files) > 20 {
		return nil, nil, errors.New("a maximum of 20 files can be uploaded at once")
	}
	stored, err := storeAssetFiles(files)
	if err != nil {
		return nil, nil, err
	}
	return appendAssetLibraryItems(ctx, userId, groupId, stored)
}

// AppendAssetLibraryURLs adds new assets to an existing group from public URLs.
func AppendAssetLibraryURLs(ctx context.Context, userId int, groupId int64, items []AssetURLInput) (*AssetLibraryGroupResponse, []AssetLibraryOperationResult, error) {
	if len(items) == 0 {
		return nil, nil, errors.New("at least one url is required")
	}
	if len(items) > 20 {
		return nil, nil, errors.New("a maximum of 20 urls can be uploaded at once")
	}
	stored := make([]storedAssetFile, 0, len(items))
	for _, item := range items {
		publicURL := strings.TrimSpace(item.URL)
		if publicURL == "" {
			return nil, nil, errors.New("url must not be empty")
		}
		parsed, parseErr := url.Parse(publicURL)
		if parseErr != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return nil, nil, errors.New("url must be a valid http or https URL")
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = filepath.Base(parsed.Path)
		}
		if name == "" || name == "." || name == "/" {
			name = publicURL
		}
		assetType := strings.TrimSpace(item.AssetType)
		if assetType == "" {
			assetType = inferAssetTypeFromName(name)
		}
		stored = append(stored, storedAssetFile{name: name, assetType: assetType, publicURL: publicURL})
	}
	return appendAssetLibraryItems(ctx, userId, groupId, stored)
}

// appendAssetLibraryItems persists the local asset rows and uploads each to
// every configured upstream.
func appendAssetLibraryItems(ctx context.Context, userId int, groupId int64, stored []storedAssetFile) (*AssetLibraryGroupResponse, []AssetLibraryOperationResult, error) {
	group, err := model.GetAssetLibraryGroup(userId, groupId)
	if err != nil {
		return nil, nil, err
	}
	upstreams, err := configuredAssetLibraryTargets()
	if err != nil {
		return nil, nil, err
	}

	assets := make([]model.AssetLibraryAsset, 0, len(stored))
	for _, item := range stored {
		assets = append(assets, model.AssetLibraryAsset{
			GroupId: group.Id, UserId: userId, Name: item.name,
			AssetType: item.assetType, SourceURL: item.publicURL,
			FileSize: item.fileSize, MimeType: item.mimeType,
		})
	}
	if err := model.CreateAssetLibraryAssets(assets); err != nil {
		return nil, nil, err
	}
	for i := range stored {
		stored[i].assetId = assets[i].Id
	}

	groupMappingByUpstream := make(map[int64]*model.AssetLibraryGroupUpstream, len(group.Mappings))
	for i := range group.Mappings {
		groupMappingByUpstream[group.Mappings[i].UpstreamId] = &group.Mappings[i]
	}

	results := make([]AssetLibraryOperationResult, 0, len(upstreams))
	for i := range upstreams {
		upstream := &upstreams[i]
		groupMapping := groupMappingByUpstream[upstream.Id]
		if groupMapping == nil || (upstream.Format == model.AssetLibraryFormatVolcengine && groupMapping.UpstreamGroupId == "") {
			// Upstream added after the group was created, or the group failed to
			// be created: (re)create it now.
			if groupMapping == nil {
				groupMapping = &model.AssetLibraryGroupUpstream{
					GroupId: group.Id, UserId: userId, UpstreamId: upstream.Id, Status: "Processing",
				}
			}
			upstreamGroupId, groupErr := createGroupOnTarget(ctx, upstream, group.DisplayName, group.GroupType)
			if groupErr != nil {
				groupMapping.Status = "Failed"
				groupMapping.ErrorMessage = groupErr.Error()
				_ = model.SaveAssetLibraryGroupUpstream(groupMapping)
				results = append(results, AssetLibraryOperationResult{
					Success: false, Message: groupErr.Error(),
				})
				continue
			}
			groupMapping.UpstreamGroupId = upstreamGroupId
			groupMapping.Status = "Active"
			groupMapping.ErrorMessage = ""
			_ = model.SaveAssetLibraryGroupUpstream(groupMapping)
		}
		result := uploadAssetsToTarget(ctx, userId, upstream, groupMapping, stored)
		results = append(results, result)
	}

	updated, err := GetAssetLibraryGroup(userId, groupId)
	return updated, results, err
}

// UpdateAssetLibraryGroup renames the group locally.
func UpdateAssetLibraryGroup(ctx context.Context, userId int, groupId int64, displayName string, description string) (*AssetLibraryGroupResponse, []AssetLibraryOperationResult, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" || len([]rune(displayName)) > 64 {
		return nil, nil, errors.New("display name must contain 1 to 64 characters")
	}
	description = strings.TrimSpace(description)
	if len([]rune(description)) > 300 {
		return nil, nil, errors.New("description must not exceed 300 characters")
	}
	group, err := model.GetAssetLibraryGroup(userId, groupId)
	if err != nil {
		return nil, nil, err
	}
	upstreamById, err := enabledUpstreamMap()
	if err != nil {
		return nil, nil, err
	}

	// Propagate the rename to every upstream that has a group mapping.
	results := make([]AssetLibraryOperationResult, 0, len(group.Mappings))
	for i := range group.Mappings {
		mapping := &group.Mappings[i]
		upstream, ok := upstreamById[mapping.UpstreamId]
		if !ok || mapping.UpstreamGroupId == "" {
			continue
		}
		reqErr := updateGroupOnTarget(ctx, upstream, mapping.UpstreamGroupId, displayName)
		result := AssetLibraryOperationResult{Success: reqErr == nil}
		if reqErr != nil {
			result.Message = reqErr.Error()
			mapping.Status = "Failed"
			mapping.ErrorMessage = reqErr.Error()
			_ = model.SaveAssetLibraryGroupUpstream(mapping)
		}
		results = append(results, result)
	}

	if err := model.UpdateAssetLibraryGroup(userId, groupId, map[string]interface{}{
		"display_name": displayName,
		"description":  description,
	}); err != nil {
		return nil, results, err
	}
	updated, err := GetAssetLibraryGroup(userId, groupId)
	return updated, results, err
}

// UpdateAssetLibraryAsset renames a single asset locally and on every upstream
// it was pushed to.
func UpdateAssetLibraryAsset(ctx context.Context, userId int, groupId int64, assetId int64, name string) (*AssetLibraryGroupResponse, []AssetLibraryOperationResult, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 64 {
		return nil, nil, errors.New("name must contain 1 to 64 characters")
	}
	group, err := model.GetAssetLibraryGroup(userId, groupId)
	if err != nil {
		return nil, nil, err
	}
	var asset *model.AssetLibraryAsset
	for i := range group.Assets {
		if group.Assets[i].Id == assetId {
			asset = &group.Assets[i]
			break
		}
	}
	if asset == nil {
		return nil, nil, errors.New("asset not found")
	}
	upstreamById, err := enabledUpstreamMap()
	if err != nil {
		return nil, nil, err
	}

	results := make([]AssetLibraryOperationResult, 0, len(asset.Mappings))
	for i := range asset.Mappings {
		mapping := &asset.Mappings[i]
		upstream, ok := upstreamById[mapping.UpstreamId]
		if !ok || mapping.UpstreamAssetId == "" {
			continue
		}
		reqErr := updateAssetOnTarget(ctx, upstream, mapping.UpstreamAssetId, name)
		result := AssetLibraryOperationResult{Success: reqErr == nil}
		if reqErr != nil {
			result.Message = reqErr.Error()
			mapping.Status = "Failed"
			mapping.ErrorMessage = reqErr.Error()
			_ = model.SaveAssetLibraryAssetUpstream(mapping)
		}
		results = append(results, result)
	}

	if err := model.UpdateAssetLibraryAsset(userId, assetId, map[string]interface{}{"name": name}); err != nil {
		return nil, results, err
	}
	updated, err := GetAssetLibraryGroup(userId, groupId)
	return updated, results, err
}

// RefreshAssetLibraryGroup polls every asset mapping so the stored status/URL
// reflects the upstream processing state.
func RefreshAssetLibraryGroup(ctx context.Context, userId int, groupId int64) (*AssetLibraryGroupResponse, []AssetLibraryOperationResult, error) {
	group, err := model.GetAssetLibraryGroup(userId, groupId)
	if err != nil {
		return nil, nil, err
	}
	upstreamById, err := enabledUpstreamMap()
	if err != nil {
		return nil, nil, err
	}

	results := make([]AssetLibraryOperationResult, 0)
	for i := range group.Assets {
		asset := &group.Assets[i]
		for j := range asset.Mappings {
			mapping := &asset.Mappings[j]
			upstream, ok := upstreamById[mapping.UpstreamId]
			if !ok || mapping.UpstreamAssetId == "" {
				continue
			}
			detail, reqErr := getAssetOnTarget(ctx, upstream, mapping.UpstreamAssetId)
			result := AssetLibraryOperationResult{Success: reqErr == nil}
			if reqErr != nil {
				result.Message = reqErr.Error()
				mapping.Status = "Failed"
				mapping.ErrorMessage = reqErr.Error()
			} else {
				mapping.Status = normalizeStatus(detail.Status)
				mapping.ErrorCode = detail.ErrorCode
				mapping.ErrorMessage = detail.ErrorMessage
				if detail.AssetURL != "" {
					mapping.AssetURL = detail.AssetURL
				}
			}
			if err := model.SaveAssetLibraryAssetUpstream(mapping); err != nil {
				return nil, results, err
			}
			results = append(results, result)
		}
	}
	updated, err := GetAssetLibraryGroup(userId, groupId)
	return updated, results, err
}

// DeleteAssetLibraryGroup deletes every asset from every upstream, then removes
// the local records.
func DeleteAssetLibraryGroup(ctx context.Context, userId int, groupId int64, force bool) ([]AssetLibraryOperationResult, error) {
	group, err := model.GetAssetLibraryGroup(userId, groupId)
	if err != nil {
		return nil, err
	}
	// A group must be empty before it can be deleted; remove its assets first.
	if len(group.Assets) > 0 {
		return nil, errors.New("素材组中还有素材，请先删除全部素材后再删除素材组")
	}
	upstreamById, err := enabledUpstreamMap()
	if err != nil {
		return nil, err
	}

	// Remove the (now empty) group from every upstream that has a group mapping.
	results := make([]AssetLibraryOperationResult, 0)
	allSucceeded := true
	for i := range group.Mappings {
		mapping := &group.Mappings[i]
		upstream, ok := upstreamById[mapping.UpstreamId]
		if mapping.UpstreamGroupId == "" {
			continue
		}
		if !ok {
			allSucceeded = false
			continue
		}
		reqErr := deleteGroupOnTarget(ctx, upstream, mapping.UpstreamGroupId)
		result := AssetLibraryOperationResult{Success: reqErr == nil}
		if reqErr != nil {
			allSucceeded = false
			result.Message = reqErr.Error()
		}
		results = append(results, result)
	}
	if !allSucceeded && !force {
		return results, errors.New("one or more upstreams failed to delete the asset group")
	}
	return results, model.DeleteAssetLibraryGroup(userId, groupId)
}

// DeleteAssetLibraryAsset deletes a single asset from every upstream, then
// removes the local record. When force is true, upstream failures are ignored
// and the local record is removed regardless.
func DeleteAssetLibraryAsset(ctx context.Context, userId int, groupId int64, assetId int64, force bool) ([]AssetLibraryOperationResult, error) {
	group, err := model.GetAssetLibraryGroup(userId, groupId)
	if err != nil {
		return nil, err
	}
	var asset *model.AssetLibraryAsset
	for i := range group.Assets {
		if group.Assets[i].Id == assetId {
			asset = &group.Assets[i]
			break
		}
	}
	if asset == nil {
		return nil, errors.New("asset not found")
	}
	upstreamById, err := enabledUpstreamMap()
	if err != nil {
		return nil, err
	}

	results := make([]AssetLibraryOperationResult, 0, len(asset.Mappings))
	allSucceeded := true
	for _, mapping := range asset.Mappings {
		upstream, ok := upstreamById[mapping.UpstreamId]
		if mapping.UpstreamAssetId == "" {
			continue
		}
		if !ok {
			allSucceeded = false
			continue
		}
		reqErr := deleteAssetOnTarget(ctx, upstream, mapping.UpstreamAssetId)
		result := AssetLibraryOperationResult{Success: reqErr == nil}
		if reqErr != nil {
			allSucceeded = false
			result.Message = reqErr.Error()
		}
		results = append(results, result)
	}
	if !allSucceeded && !force {
		return results, errors.New("one or more upstreams failed to delete the asset")
	}
	removeLocalAssetFile(asset.SourceURL)
	return results, model.DeleteAssetLibraryAsset(userId, groupId, assetId)
}

func enabledUpstreamMap() (map[int64]*model.AssetLibraryUpstream, error) {
	upstreams, err := configuredAssetLibraryTargets()
	if err != nil {
		return nil, err
	}
	byId := make(map[int64]*model.AssetLibraryUpstream, len(upstreams))
	for i := range upstreams {
		byId[upstreams[i].Id] = &upstreams[i]
	}
	return byId, nil
}

// ---- local file storage ----

// storedAssetFile is a client asset persisted locally (for file uploads) and/or
// referenced by a public URL (for URL imports), ready to push to upstreams.
type storedAssetFile struct {
	name      string
	assetType string
	publicURL string
	localPath string
	mimeType  string
	fileSize  int64
	assetId   int64
}

func storeAssetFiles(files []*multipart.FileHeader) ([]storedAssetFile, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(system_setting.ServerAddress), "/")
	if err := os.MkdirAll(assetLibraryUploadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create asset upload directory: %w", err)
	}
	stored := make([]storedAssetFile, 0, len(files))
	for _, fileHeader := range files {
		ext := filepath.Ext(fileHeader.Filename)
		filename := uuid.New().String() + ext
		destPath := filepath.Join(assetLibraryUploadDir, filename)
		if err := saveMultipartFile(fileHeader, destPath); err != nil {
			return nil, err
		}
		mimeType := fileHeader.Header.Get("Content-Type")
		item := storedAssetFile{
			name:      fileHeader.Filename,
			assetType: inferAssetType(fileHeader),
			localPath: destPath,
			mimeType:  mimeType,
			fileSize:  fileHeader.Size,
		}
		// Only expose a public URL when the server address is configured; some
		// upstreams (volcengine) require it, but OpenAI uploads the bytes directly.
		if baseURL != "" {
			item.publicURL = baseURL + assetLibraryURLPrefix + filename
		}
		stored = append(stored, item)
	}
	return stored, nil
}

func saveMultipartFile(fileHeader *multipart.FileHeader, destPath string) error {
	src, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	return nil
}

func removeLocalAssetFile(sourceURL string) {
	if sourceURL == "" {
		return
	}
	idx := strings.Index(sourceURL, assetLibraryURLPrefix)
	if idx < 0 {
		return
	}
	filename := sourceURL[idx+len(assetLibraryURLPrefix):]
	if filename == "" || strings.ContainsAny(filename, "/\\") {
		return
	}
	_ = os.Remove(filepath.Join(assetLibraryUploadDir, filename))
}

// ---- per-target dispatch (format-aware) ----

// createGroupOnTarget creates the upstream asset group. OpenAI upstreams have no
// group concept and return an empty id.
func createGroupOnTarget(ctx context.Context, upstream *model.AssetLibraryUpstream, name string, groupType string) (string, error) {
	switch upstream.Format {
	case model.AssetLibraryFormatOpenAI:
		return "", nil
	default:
		return createVolcGroup(ctx, upstream, name, groupType)
	}
}

// deleteGroupOnTarget removes a group from the upstream. OpenAI has no group
// concept, so it is a no-op there.
func deleteGroupOnTarget(ctx context.Context, upstream *model.AssetLibraryUpstream, upstreamGroupId string) error {
	switch upstream.Format {
	case model.AssetLibraryFormatOpenAI:
		return nil
	default:
		return deleteVolcGroup(ctx, upstream, upstreamGroupId)
	}
}

func uploadAssetsToTarget(ctx context.Context, userId int, upstream *model.AssetLibraryUpstream, groupMapping *model.AssetLibraryGroupUpstream, items []storedAssetFile) AssetLibraryOperationResult {
	result := AssetLibraryOperationResult{Success: true}
	for _, item := range items {
		mapping := &model.AssetLibraryAssetUpstream{
			AssetId: item.assetId, GroupUpstreamId: groupMapping.Id, UserId: userId,
			UpstreamId: upstream.Id, AssetURL: item.publicURL,
		}
		upstreamResult, uploadErr := createAssetOnTarget(ctx, upstream, groupMapping.UpstreamGroupId, item)
		if uploadErr != nil {
			mapping.Status = "Failed"
			mapping.ErrorMessage = uploadErr.Error()
			result.Success = false
			if result.Message == "" {
				result.Message = uploadErr.Error()
			}
		} else {
			mapping.UpstreamAssetId = upstreamResult.UpstreamAssetId
			mapping.Status = normalizeStatus(upstreamResult.Status)
			if upstreamResult.AssetURL != "" {
				mapping.AssetURL = upstreamResult.AssetURL
			}
		}
		if err := model.SaveAssetLibraryAssetUpstream(mapping); err != nil {
			result.Success = false
			result.Message = err.Error()
		}
	}
	return result
}

func createAssetOnTarget(ctx context.Context, upstream *model.AssetLibraryUpstream, upstreamGroupId string, item storedAssetFile) (*upstreamAssetResult, error) {
	switch upstream.Format {
	case model.AssetLibraryFormatOpenAI:
		return createOpenAIFile(ctx, upstream, item)
	default:
		return createVolcAsset(ctx, upstream, upstreamGroupId, item)
	}
}

func getAssetOnTarget(ctx context.Context, upstream *model.AssetLibraryUpstream, upstreamAssetId string) (*upstreamAssetResult, error) {
	switch upstream.Format {
	case model.AssetLibraryFormatOpenAI:
		return getOpenAIFile(ctx, upstream, upstreamAssetId)
	default:
		return getVolcAsset(ctx, upstream, upstreamAssetId)
	}
}

func deleteAssetOnTarget(ctx context.Context, upstream *model.AssetLibraryUpstream, upstreamAssetId string) error {
	switch upstream.Format {
	case model.AssetLibraryFormatOpenAI:
		return deleteOpenAIFile(ctx, upstream, upstreamAssetId)
	default:
		return deleteVolcAsset(ctx, upstream, upstreamAssetId)
	}
}

// updateAssetOnTarget renames an asset on the upstream. OpenAI's Files API has no
// rename operation, so it is a no-op there.
func updateAssetOnTarget(ctx context.Context, upstream *model.AssetLibraryUpstream, upstreamAssetId string, name string) error {
	switch upstream.Format {
	case model.AssetLibraryFormatOpenAI:
		return nil
	default:
		return updateVolcAsset(ctx, upstream, upstreamAssetId, name)
	}
}

// updateGroupOnTarget renames a group on the upstream. OpenAI has no group
// concept, so it is a no-op there.
func updateGroupOnTarget(ctx context.Context, upstream *model.AssetLibraryUpstream, upstreamGroupId string, name string) error {
	switch upstream.Format {
	case model.AssetLibraryFormatOpenAI:
		return nil
	default:
		return updateVolcGroup(ctx, upstream, upstreamGroupId, name)
	}
}

// ---- volcengine implementation ----

func createVolcGroup(ctx context.Context, upstream *model.AssetLibraryUpstream, name string, groupType string) (string, error) {
	payload := map[string]interface{}{"Name": name, "GroupType": groupType}
	if upstream.ProjectName != "" {
		payload["ProjectName"] = upstream.ProjectName
	}
	var result volcIdResult
	action := firstNonEmpty(upstream.CreateGroupAction, assetVolcDefaultCreateGroupAction)
	if err := doVolcAction(ctx, upstream, action, payload, &result); err != nil {
		return "", err
	}
	if result.Id == "" {
		return "", errors.New("upstream did not return a group id")
	}
	return result.Id, nil
}

func createVolcAsset(ctx context.Context, upstream *model.AssetLibraryUpstream, upstreamGroupId string, item storedAssetFile) (*upstreamAssetResult, error) {
	if item.publicURL == "" {
		return nil, errors.New("server address is not configured; cannot expose a public URL for the volcengine upstream")
	}
	payload := map[string]interface{}{
		"GroupId":   upstreamGroupId,
		"URL":       item.publicURL,
		"AssetType": item.assetType,
		"Name":      truncateAssetName(item.name),
	}
	if upstream.ProjectName != "" {
		payload["ProjectName"] = upstream.ProjectName
	}
	var result volcIdResult
	action := firstNonEmpty(upstream.CreateAssetAction, assetVolcDefaultCreateAssetAction)
	if err := doVolcAction(ctx, upstream, action, payload, &result); err != nil {
		return nil, err
	}
	if result.Id == "" {
		return nil, errors.New("upstream did not return an asset id")
	}
	// CreateAsset is async; the asset stays Processing until GetAsset confirms it.
	return &upstreamAssetResult{UpstreamAssetId: result.Id, Status: "Processing"}, nil
}

func getVolcAsset(ctx context.Context, upstream *model.AssetLibraryUpstream, upstreamAssetId string) (*upstreamAssetResult, error) {
	payload := map[string]interface{}{"Id": upstreamAssetId}
	if upstream.ProjectName != "" {
		payload["ProjectName"] = upstream.ProjectName
	}
	var result volcGetAssetResult
	action := firstNonEmpty(upstream.GetAssetAction, assetVolcDefaultGetAssetAction)
	if err := doVolcAction(ctx, upstream, action, payload, &result); err != nil {
		return nil, err
	}
	return &upstreamAssetResult{
		UpstreamAssetId: result.Id, AssetURL: result.URL, Status: result.Status,
		ErrorCode: result.ErrorCode, ErrorMessage: result.ErrorMessage,
	}, nil
}

func deleteVolcAsset(ctx context.Context, upstream *model.AssetLibraryUpstream, upstreamAssetId string) error {
	payload := map[string]interface{}{"Id": upstreamAssetId}
	if upstream.ProjectName != "" {
		payload["ProjectName"] = upstream.ProjectName
	}
	action := firstNonEmpty(upstream.DeleteAssetAction, assetVolcDefaultDeleteAssetAction)
	return doVolcAction(ctx, upstream, action, payload, nil)
}

func updateVolcAsset(ctx context.Context, upstream *model.AssetLibraryUpstream, upstreamAssetId string, name string) error {
	payload := map[string]interface{}{"Id": upstreamAssetId, "Name": truncateAssetName(name)}
	if upstream.ProjectName != "" {
		payload["ProjectName"] = upstream.ProjectName
	}
	action := firstNonEmpty(upstream.UpdateAssetAction, assetVolcDefaultUpdateAssetAction)
	return doVolcAction(ctx, upstream, action, payload, nil)
}

func updateVolcGroup(ctx context.Context, upstream *model.AssetLibraryUpstream, upstreamGroupId string, name string) error {
	payload := map[string]interface{}{"Id": upstreamGroupId, "Name": name}
	if upstream.ProjectName != "" {
		payload["ProjectName"] = upstream.ProjectName
	}
	action := firstNonEmpty(upstream.UpdateGroupAction, assetVolcDefaultUpdateGroupAction)
	return doVolcAction(ctx, upstream, action, payload, nil)
}

func deleteVolcGroup(ctx context.Context, upstream *model.AssetLibraryUpstream, upstreamGroupId string) error {
	payload := map[string]interface{}{"Id": upstreamGroupId}
	if upstream.ProjectName != "" {
		payload["ProjectName"] = upstream.ProjectName
	}
	action := firstNonEmpty(upstream.DeleteGroupAction, assetVolcDefaultDeleteGroupAction)
	return doVolcAction(ctx, upstream, action, payload, nil)
}

// doVolcAction performs one volcengine query-action request and decodes the
// {"ResponseMetadata":{...},"Result":{...}} envelope.
func doVolcAction(ctx context.Context, upstream *model.AssetLibraryUpstream, action string, payload interface{}, output interface{}) error {
	if strings.TrimSpace(action) == "" {
		return errors.New("asset action is not configured")
	}
	requestURL, err := buildVolcActionURL(upstream, action)
	if err != nil {
		return err
	}
	var body io.Reader
	if payload != nil {
		encoded, err := common.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+upstream.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := GetHttpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, assetLibraryResponseLimit))
	if err != nil {
		return err
	}

	var envelope volcEnvelope
	parseErr := common.Unmarshal(responseBody, &envelope)
	if parseErr == nil && envelope.ResponseMetadata.Error != nil {
		return volcBusinessError(envelope.ResponseMetadata.Error)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return upstreamHTTPError(resp.StatusCode, responseBody)
	}
	if parseErr != nil {
		return parseErr
	}
	if output != nil && len(envelope.Result) > 0 && string(envelope.Result) != "null" {
		if err := common.Unmarshal(envelope.Result, output); err != nil {
			return err
		}
	}
	return nil
}

func buildVolcActionURL(upstream *model.AssetLibraryUpstream, action string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(upstream.BaseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("asset library base URL is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("asset library base URL must use http or https")
	}
	query := parsed.Query()
	query.Set("Action", action)
	query.Set("Version", firstNonEmpty(upstream.Version, assetVolcDefaultVersion))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func volcBusinessError(e *volcError) error {
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = strings.TrimSpace(e.Code)
	}
	if message == "" {
		message = "upstream asset operation failed"
	}
	return errors.New(message)
}

// ---- openai Files API implementation ----

func createOpenAIFile(ctx context.Context, upstream *model.AssetLibraryUpstream, item storedAssetFile) (*upstreamAssetResult, error) {
	reader, filename, err := openAssetContent(item)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	purpose := firstNonEmpty(upstream.Purpose, assetOpenAIDefaultPurpose)
	if err := writer.WriteField("purpose", purpose); err != nil {
		return nil, err
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, reader); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	requestURL, err := buildOpenAIURL(upstream, "files")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, &requestBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+upstream.APIKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := GetHttpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, assetLibraryResponseLimit))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, openAIError(resp.StatusCode, responseBody)
	}
	var file openAIFileObject
	if err := common.Unmarshal(responseBody, &file); err != nil {
		return nil, err
	}
	if file.Id == "" {
		return nil, errors.New("upstream did not return a file id")
	}
	// Files are immediately usable; treat as Active unless the upstream reports otherwise.
	return &upstreamAssetResult{UpstreamAssetId: file.Id, Status: normalizeOpenAIStatus(file.Status), AssetURL: item.publicURL}, nil
}

func getOpenAIFile(ctx context.Context, upstream *model.AssetLibraryUpstream, fileId string) (*upstreamAssetResult, error) {
	requestURL, err := buildOpenAIURL(upstream, "files/"+url.PathEscape(fileId))
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+upstream.APIKey)
	resp, err := GetHttpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, assetLibraryResponseLimit))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, openAIError(resp.StatusCode, responseBody)
	}
	var file openAIFileObject
	if err := common.Unmarshal(responseBody, &file); err != nil {
		return nil, err
	}
	return &upstreamAssetResult{UpstreamAssetId: file.Id, Status: normalizeOpenAIStatus(file.Status)}, nil
}

func deleteOpenAIFile(ctx context.Context, upstream *model.AssetLibraryUpstream, fileId string) error {
	requestURL, err := buildOpenAIURL(upstream, "files/"+url.PathEscape(fileId))
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, requestURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+upstream.APIKey)
	resp, err := GetHttpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, assetLibraryResponseLimit))
	if err != nil {
		return err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return openAIError(resp.StatusCode, responseBody)
	}
	return nil
}

func buildOpenAIURL(upstream *model.AssetLibraryUpstream, path string) (string, error) {
	base := strings.TrimSpace(upstream.BaseURL)
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("asset library base URL is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("asset library base URL must use http or https")
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/"), nil
}

func normalizeOpenAIStatus(status string) string {
	switch strings.ToLower(status) {
	case "processed", "uploaded", "":
		return "Active"
	case "error":
		return "Failed"
	default:
		return status
	}
}

func openAIError(status int, body []byte) error {
	var envelope openAIErrorEnvelope
	if err := common.Unmarshal(body, &envelope); err == nil && envelope.Error != nil && envelope.Error.Message != "" {
		return fmt.Errorf("upstream returned %d: %s", status, envelope.Error.Message)
	}
	return upstreamHTTPError(status, body)
}

// openAssetContent opens the asset's bytes: the stored local file when present,
// otherwise the public URL (for URL imports).
func openAssetContent(item storedAssetFile) (io.ReadCloser, string, error) {
	if item.localPath != "" {
		f, err := os.Open(item.localPath)
		if err != nil {
			return nil, "", err
		}
		return f, item.name, nil
	}
	if item.publicURL == "" {
		return nil, "", errors.New("asset has no local file or public URL")
	}
	resp, err := GetHttpClient().Get(item.publicURL)
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		resp.Body.Close()
		return nil, "", fmt.Errorf("failed to download asset URL: status %d", resp.StatusCode)
	}
	name := item.name
	if name == "" {
		name = filepath.Base(item.publicURL)
	}
	return resp.Body, name, nil
}

// ---- shared helpers ----

func upstreamHTTPError(status int, body []byte) error {
	message := strings.TrimSpace(string(body))
	if len(message) > 300 {
		message = message[:300]
	}
	if message == "" {
		message = http.StatusText(status)
	}
	return fmt.Errorf("upstream returned %d: %s", status, message)
}

func normalizeStatus(status string) string {
	if status == "" {
		return "Processing"
	}
	return status
}

func truncateAssetName(name string) string {
	runes := []rune(name)
	if len(runes) > 64 {
		return string(runes[:64])
	}
	return name
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func toGroupResponse(group *model.AssetLibraryGroup) AssetLibraryGroupResponse {
	resp := AssetLibraryGroupResponse{
		Id:          group.Id,
		DisplayName: group.DisplayName,
		Description: group.Description,
		GroupType:   group.GroupType,
		CoverURL:    group.CoverURL,
		CreatedTime: group.CreatedTime,
		UpdatedTime: group.UpdatedTime,
		Status:      aggregateGroupStatus(group.Mappings),
		Assets:      make([]AssetLibraryAssetResponse, 0, len(group.Assets)),
	}
	for i := range group.Assets {
		resp.Assets = append(resp.Assets, toAssetResponse(&group.Assets[i]))
	}
	return resp
}

func toAssetResponse(asset *model.AssetLibraryAsset) AssetLibraryAssetResponse {
	resp := AssetLibraryAssetResponse{
		Id:          asset.Id,
		GroupId:     asset.GroupId,
		Name:        asset.Name,
		AssetType:   asset.AssetType,
		SourceURL:   asset.SourceURL,
		FileSize:    asset.FileSize,
		MimeType:    asset.MimeType,
		CreatedTime: asset.CreatedTime,
		UpdatedTime: asset.UpdatedTime,
	}
	for i := range asset.Mappings {
		m := &asset.Mappings[i]
		if resp.AssetURL == "" && m.AssetURL != "" {
			resp.AssetURL = m.AssetURL
		}
		if resp.AssetId == "" && m.UpstreamAssetId != "" {
			resp.AssetId = m.UpstreamAssetId
		}
	}
	resp.Status = aggregateAssetStatus(asset.Mappings)
	return resp
}

func aggregateGroupStatus(mappings []model.AssetLibraryGroupUpstream) string {
	if len(mappings) == 0 {
		return "Processing"
	}
	for _, m := range mappings {
		if m.Status == "Failed" {
			return "Failed"
		}
	}
	for _, m := range mappings {
		if m.Status == "Processing" || m.Status == "" {
			return "Processing"
		}
	}
	return "Active"
}

func aggregateAssetStatus(mappings []model.AssetLibraryAssetUpstream) string {
	if len(mappings) == 0 {
		return "Processing"
	}
	for _, m := range mappings {
		if m.Status == "Failed" {
			return "Failed"
		}
	}
	for _, m := range mappings {
		if m.Status == "Processing" || m.Status == "" {
			return "Processing"
		}
	}
	return "Active"
}

func inferAssetType(file *multipart.FileHeader) string {
	contentType := strings.ToLower(file.Header.Get("Content-Type"))
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return "Image"
	case strings.HasPrefix(contentType, "audio/"):
		return "Audio"
	case strings.HasPrefix(contentType, "video/"):
		return "Video"
	}
	return inferAssetTypeFromName(file.Filename)
}

// AssetURLInput describes a single public-URL asset to import into a group.
type AssetURLInput struct {
	URL       string `json:"url"`
	Name      string `json:"name"`
	AssetType string `json:"asset_type"`
}

// inferAssetTypeFromName guesses the asset type from a file name/URL extension.
func inferAssetTypeFromName(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp", ".tiff", ".heic", ".heif":
		return "Image"
	case ".mp3", ".wav", ".aac", ".m4a", ".flac", ".ogg":
		return "Audio"
	default:
		return "Video"
	}
}
