package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

const assetLibraryResponseLimit = 8 << 20

type AssetLibraryChannel struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type upstreamAsset struct {
	AssetId      string `json:"assetId"`
	AssetName    string `json:"assetName"`
	AssetURL     string `json:"assetUrl"`
	AssetType    string `json:"assetType"`
	Status       string `json:"status"`
	ErrorCode    string `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
}

type upstreamGroup struct {
	GroupId      string          `json:"groupId"`
	DisplayName  string          `json:"displayName"`
	Description  string          `json:"description"`
	GroupType    string          `json:"groupType"`
	CoverAssetId string          `json:"coverAssetId"`
	CoverURL     string          `json:"coverUrl"`
	Assets       []upstreamAsset `json:"assets"`
}

type upstreamBatch struct {
	GroupId string          `json:"groupId"`
	Assets  []upstreamAsset `json:"assets"`
}

type upstreamError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type upstreamGroupEnvelope struct {
	Data      upstreamGroup  `json:"data"`
	Error     *upstreamError `json:"error"`
	RequestId string         `json:"request_id"`
}

type upstreamBatchEnvelope struct {
	Data      upstreamBatch  `json:"data"`
	Error     *upstreamError `json:"error"`
	RequestId string         `json:"request_id"`
}

type assetChannelTarget struct {
	Channel *model.Channel
	Config  *dto.AssetLibraryEndpointSettings
}

type AssetLibraryOperationResult struct {
	ChannelId   int    `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	Success     bool   `json:"success"`
	Message     string `json:"message,omitempty"`
}

func configuredAssetLibraryTargets() ([]assetChannelTarget, error) {
	channels, err := model.GetAssetLibraryChannels()
	if err != nil {
		return nil, err
	}
	targets := make([]assetChannelTarget, 0)
	for _, channel := range channels {
		settings := channel.GetOtherSettings()
		if settings.AssetLibrary == nil || !settings.AssetLibrary.Enabled {
			continue
		}
		targets = append(targets, assetChannelTarget{Channel: channel, Config: settings.AssetLibrary})
	}
	return targets, nil
}

func ListAssetLibraryChannels() ([]AssetLibraryChannel, error) {
	targets, err := configuredAssetLibraryTargets()
	if err != nil {
		return nil, err
	}
	channels := make([]AssetLibraryChannel, 0, len(targets))
	for _, target := range targets {
		channels = append(channels, AssetLibraryChannel{Id: target.Channel.Id, Name: target.Channel.Name})
	}
	return channels, nil
}

func ListAssetLibraryGroups(userId int) ([]model.AssetLibraryGroup, error) {
	groups, err := model.ListAssetLibraryGroups(userId)
	if err != nil {
		return nil, err
	}
	decorateAssetLibraryGroups(groups)
	return groups, nil
}

func GetAssetLibraryGroup(userId int, groupId int64) (*model.AssetLibraryGroup, error) {
	group, err := model.GetAssetLibraryGroup(userId, groupId)
	if err != nil {
		return nil, err
	}
	decorateAssetLibraryGroup(group)
	return group, nil
}

func CreateAssetLibraryGroup(ctx context.Context, userId int, displayName string, files []*multipart.FileHeader) (*model.AssetLibraryGroup, []AssetLibraryOperationResult, error) {
	if len(files) == 0 {
		return nil, nil, errors.New("at least one file is required")
	}
	if len(files) > 20 {
		return nil, nil, errors.New("a maximum of 20 files can be uploaded at once")
	}
	if strings.TrimSpace(displayName) == "" {
		displayName = strings.TrimSuffix(files[0].Filename, filepath.Ext(files[0].Filename))
	}
	if len([]rune(displayName)) > 64 {
		return nil, nil, errors.New("display name must not exceed 64 characters")
	}

	targets, err := configuredAssetLibraryTargets()
	if err != nil {
		return nil, nil, err
	}
	if len(targets) == 0 {
		return nil, nil, errors.New("no asset library channel is configured")
	}

	group := &model.AssetLibraryGroup{
		UserId:      userId,
		DisplayName: displayName,
		GroupType:   "AIGC",
		Assets:      make([]model.AssetLibraryAsset, 0, len(files)),
	}
	for _, file := range files {
		group.Assets = append(group.Assets, model.AssetLibraryAsset{
			Name:      file.Filename,
			AssetType: inferAssetType(file),
		})
	}
	if err := model.CreateAssetLibraryGroup(group); err != nil {
		return nil, nil, err
	}

	results := make([]AssetLibraryOperationResult, 0, len(targets))
	for _, target := range targets {
		upstream, uploadErr := uploadAssetFiles(ctx, target, target.Config.CreatePath, displayName, files)
		mapping := &model.AssetLibraryGroupChannel{
			GroupId:   group.Id,
			UserId:    userId,
			ChannelId: target.Channel.Id,
			Status:    "Active",
		}
		result := AssetLibraryOperationResult{ChannelId: target.Channel.Id, ChannelName: target.Channel.Name, Success: uploadErr == nil}
		if uploadErr != nil {
			mapping.Status = "Failed"
			mapping.ErrorMessage = uploadErr.Error()
			result.Message = uploadErr.Error()
		} else {
			mapping.UpstreamGroupId = upstream.GroupId
			if group.Description == "" {
				group.Description = upstream.Description
			}
			if upstream.GroupType != "" {
				group.GroupType = upstream.GroupType
			}
			if group.CoverURL == "" {
				group.CoverURL = upstream.CoverURL
			}
		}
		if saveErr := model.SaveAssetLibraryGroupChannel(mapping); saveErr != nil {
			return nil, results, saveErr
		}
		if uploadErr == nil {
			if err := saveReturnedAssetMappings(userId, target, mapping, group.Assets, upstream.Assets); err != nil {
				return nil, results, err
			}
		} else if err := saveFailedAssetMappings(userId, target, mapping, group.Assets, uploadErr); err != nil {
			return nil, results, err
		}
		results = append(results, result)
	}
	if err := model.UpdateAssetLibraryGroup(userId, group.Id, map[string]interface{}{
		"description": group.Description,
		"group_type":  group.GroupType,
		"cover_url":   group.CoverURL,
	}); err != nil {
		return nil, results, err
	}
	created, err := GetAssetLibraryGroup(userId, group.Id)
	return created, results, err
}

func AppendAssetLibraryFiles(ctx context.Context, userId int, groupId int64, files []*multipart.FileHeader) (*model.AssetLibraryGroup, []AssetLibraryOperationResult, error) {
	if len(files) == 0 {
		return nil, nil, errors.New("at least one file is required")
	}
	if len(files) > 20 {
		return nil, nil, errors.New("a maximum of 20 files can be uploaded at once")
	}
	group, err := model.GetAssetLibraryGroup(userId, groupId)
	if err != nil {
		return nil, nil, err
	}
	targets, err := configuredAssetLibraryTargets()
	if err != nil {
		return nil, nil, err
	}
	assets := make([]model.AssetLibraryAsset, 0, len(files))
	for _, file := range files {
		assets = append(assets, model.AssetLibraryAsset{
			GroupId: group.Id, UserId: userId, Name: file.Filename, AssetType: inferAssetType(file),
		})
	}
	if err := model.CreateAssetLibraryAssets(assets); err != nil {
		return nil, nil, err
	}

	mappingByChannel := make(map[int]*model.AssetLibraryGroupChannel, len(group.Mappings))
	for i := range group.Mappings {
		mappingByChannel[group.Mappings[i].ChannelId] = &group.Mappings[i]
	}
	results := make([]AssetLibraryOperationResult, 0, len(targets))
	for _, target := range targets {
		groupMapping := mappingByChannel[target.Channel.Id]
		endpointPath := target.Config.CreatePath
		uploadDisplayName := group.DisplayName
		if groupMapping == nil {
			groupMapping = &model.AssetLibraryGroupChannel{
				GroupId: group.Id, UserId: userId, ChannelId: target.Channel.Id,
			}
		} else if groupMapping.UpstreamGroupId != "" {
			endpointPath = expandAssetPath(target.Config.AppendPath, groupMapping.UpstreamGroupId, "")
			uploadDisplayName = ""
		}
		upstream, uploadErr := uploadAssetFiles(ctx, target, endpointPath, uploadDisplayName, files)
		result := AssetLibraryOperationResult{ChannelId: target.Channel.Id, ChannelName: target.Channel.Name, Success: uploadErr == nil}
		if uploadErr != nil {
			result.Message = uploadErr.Error()
			groupMapping.Status = "Failed"
			groupMapping.ErrorMessage = uploadErr.Error()
		} else {
			groupMapping.Status = "Active"
			groupMapping.ErrorMessage = ""
			groupMapping.UpstreamGroupId = upstream.GroupId
		}
		if err := model.SaveAssetLibraryGroupChannel(groupMapping); err != nil {
			return nil, results, err
		}
		if uploadErr == nil {
			if err := saveReturnedAssetMappings(userId, target, groupMapping, assets, upstream.Assets); err != nil {
				return nil, results, err
			}
		} else if err := saveFailedAssetMappings(userId, target, groupMapping, assets, uploadErr); err != nil {
			return nil, results, err
		}
		results = append(results, result)
	}
	updated, err := GetAssetLibraryGroup(userId, groupId)
	return updated, results, err
}

func UpdateAssetLibraryGroup(ctx context.Context, userId int, groupId int64, displayName string) (*model.AssetLibraryGroup, []AssetLibraryOperationResult, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" || len([]rune(displayName)) > 64 {
		return nil, nil, errors.New("display name must contain 1 to 64 characters")
	}
	group, err := model.GetAssetLibraryGroup(userId, groupId)
	if err != nil {
		return nil, nil, err
	}
	targets, err := configuredAssetLibraryTargets()
	if err != nil {
		return nil, nil, err
	}
	targetById := make(map[int]assetChannelTarget, len(targets))
	for _, target := range targets {
		targetById[target.Channel.Id] = target
	}
	payload := map[string]string{"displayName": displayName}
	results := make([]AssetLibraryOperationResult, 0, len(group.Mappings))
	for i := range group.Mappings {
		mapping := &group.Mappings[i]
		target, ok := targetById[mapping.ChannelId]
		if !ok || mapping.UpstreamGroupId == "" {
			continue
		}
		path := expandAssetPath(target.Config.DetailPath, mapping.UpstreamGroupId, "")
		err := doAssetJSON(ctx, target, http.MethodPatch, path, payload, nil)
		result := AssetLibraryOperationResult{ChannelId: target.Channel.Id, ChannelName: target.Channel.Name, Success: err == nil}
		if err != nil {
			result.Message = err.Error()
			mapping.Status = "Failed"
			mapping.ErrorMessage = err.Error()
		} else {
			mapping.Status = "Active"
			mapping.ErrorMessage = ""
		}
		if saveErr := model.SaveAssetLibraryGroupChannel(mapping); saveErr != nil {
			return nil, results, saveErr
		}
		results = append(results, result)
	}
	if err := model.UpdateAssetLibraryGroup(userId, groupId, map[string]interface{}{"display_name": displayName}); err != nil {
		return nil, results, err
	}
	updated, err := GetAssetLibraryGroup(userId, groupId)
	return updated, results, err
}

func RefreshAssetLibraryGroup(ctx context.Context, userId int, groupId int64) (*model.AssetLibraryGroup, []AssetLibraryOperationResult, error) {
	group, err := model.GetAssetLibraryGroup(userId, groupId)
	if err != nil {
		return nil, nil, err
	}
	targets, err := configuredAssetLibraryTargets()
	if err != nil {
		return nil, nil, err
	}
	targetById := make(map[int]assetChannelTarget, len(targets))
	for _, target := range targets {
		targetById[target.Channel.Id] = target
	}
	results := make([]AssetLibraryOperationResult, 0, len(group.Mappings))
	for i := range group.Mappings {
		mapping := &group.Mappings[i]
		target, ok := targetById[mapping.ChannelId]
		if !ok || mapping.UpstreamGroupId == "" {
			continue
		}
		path := expandAssetPath(target.Config.DetailPath, mapping.UpstreamGroupId, "")
		var envelope upstreamGroupEnvelope
		requestErr := doAssetJSON(ctx, target, http.MethodGet, path, nil, &envelope)
		result := AssetLibraryOperationResult{ChannelId: target.Channel.Id, ChannelName: target.Channel.Name, Success: requestErr == nil}
		if requestErr != nil {
			result.Message = requestErr.Error()
			mapping.Status = "Failed"
			mapping.ErrorMessage = requestErr.Error()
		} else {
			mapping.Status = "Active"
			mapping.ErrorMessage = ""
			if envelope.Data.CoverURL != "" {
				_ = model.UpdateAssetLibraryGroup(userId, groupId, map[string]interface{}{"cover_url": envelope.Data.CoverURL})
			}
			if err := updateReturnedAssetMappings(userId, target, mapping, group.Assets, envelope.Data.Assets); err != nil {
				return nil, results, err
			}
		}
		if err := model.SaveAssetLibraryGroupChannel(mapping); err != nil {
			return nil, results, err
		}
		results = append(results, result)
	}
	updated, err := GetAssetLibraryGroup(userId, groupId)
	return updated, results, err
}

func DeleteAssetLibraryGroup(ctx context.Context, userId int, groupId int64) ([]AssetLibraryOperationResult, error) {
	group, err := model.GetAssetLibraryGroup(userId, groupId)
	if err != nil {
		return nil, err
	}
	targets, err := configuredAssetLibraryTargets()
	if err != nil {
		return nil, err
	}
	targetById := make(map[int]assetChannelTarget, len(targets))
	for _, target := range targets {
		targetById[target.Channel.Id] = target
	}
	results := make([]AssetLibraryOperationResult, 0, len(group.Mappings))
	allSucceeded := true
	for i := range group.Mappings {
		mapping := &group.Mappings[i]
		target, ok := targetById[mapping.ChannelId]
		if mapping.UpstreamGroupId == "" {
			continue
		}
		if !ok {
			allSucceeded = false
			continue
		}
		path := expandAssetPath(target.Config.DetailPath, mapping.UpstreamGroupId, "")
		requestErr := doAssetJSON(ctx, target, http.MethodDelete, path, nil, nil)
		result := AssetLibraryOperationResult{ChannelId: target.Channel.Id, ChannelName: target.Channel.Name, Success: requestErr == nil}
		if requestErr != nil {
			allSucceeded = false
			result.Message = requestErr.Error()
			mapping.Status = "Failed"
			mapping.ErrorMessage = requestErr.Error()
			_ = model.SaveAssetLibraryGroupChannel(mapping)
		}
		results = append(results, result)
	}
	if !allSucceeded {
		return results, errors.New("one or more upstream channels failed to delete the asset group")
	}
	return results, model.DeleteAssetLibraryGroup(userId, groupId)
}

func DeleteAssetLibraryAsset(ctx context.Context, userId int, groupId int64, assetId int64) ([]AssetLibraryOperationResult, error) {
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
	targets, err := configuredAssetLibraryTargets()
	if err != nil {
		return nil, err
	}
	targetById := make(map[int]assetChannelTarget, len(targets))
	groupMappingByChannel := make(map[int]model.AssetLibraryGroupChannel, len(group.Mappings))
	for _, target := range targets {
		targetById[target.Channel.Id] = target
	}
	for _, mapping := range group.Mappings {
		groupMappingByChannel[mapping.ChannelId] = mapping
	}
	results := make([]AssetLibraryOperationResult, 0, len(asset.Mappings))
	allSucceeded := true
	for _, mapping := range asset.Mappings {
		target, ok := targetById[mapping.ChannelId]
		groupMapping, hasGroupMapping := groupMappingByChannel[mapping.ChannelId]
		if mapping.UpstreamAssetId == "" {
			continue
		}
		if !ok || !hasGroupMapping || groupMapping.UpstreamGroupId == "" {
			allSucceeded = false
			continue
		}
		path := expandAssetPath(target.Config.DeleteAssetPath, groupMapping.UpstreamGroupId, mapping.UpstreamAssetId)
		requestErr := doAssetJSON(ctx, target, http.MethodDelete, path, nil, nil)
		result := AssetLibraryOperationResult{ChannelId: target.Channel.Id, ChannelName: target.Channel.Name, Success: requestErr == nil}
		if requestErr != nil {
			allSucceeded = false
			result.Message = requestErr.Error()
			mapping.Status = "Failed"
			mapping.ErrorMessage = requestErr.Error()
			_ = model.SaveAssetLibraryAssetChannel(&mapping)
		}
		results = append(results, result)
	}
	if !allSucceeded {
		return results, errors.New("one or more upstream channels failed to delete the asset")
	}
	return results, model.DeleteAssetLibraryAsset(userId, groupId, assetId)
}

func decorateAssetLibraryGroups(groups []model.AssetLibraryGroup) {
	for i := range groups {
		decorateAssetLibraryGroup(&groups[i])
	}
}

func decorateAssetLibraryGroup(group *model.AssetLibraryGroup) {
	if group.Assets == nil {
		group.Assets = []model.AssetLibraryAsset{}
	}
	if group.Mappings == nil {
		group.Mappings = []model.AssetLibraryGroupChannel{}
	}
	for i := range group.Assets {
		if group.Assets[i].Mappings == nil {
			group.Assets[i].Mappings = []model.AssetLibraryAssetChannel{}
		}
	}
	ids := make(map[int]struct{})
	for _, mapping := range group.Mappings {
		ids[mapping.ChannelId] = struct{}{}
	}
	for _, asset := range group.Assets {
		for _, mapping := range asset.Mappings {
			ids[mapping.ChannelId] = struct{}{}
		}
	}
	channelIds := make([]int, 0, len(ids))
	for id := range ids {
		channelIds = append(channelIds, id)
	}
	if len(channelIds) == 0 {
		return
	}
	channels, _ := model.GetChannelsByIds(channelIds)
	names := make(map[int]string, len(channels))
	for _, channel := range channels {
		names[channel.Id] = channel.Name
	}
	for i := range group.Mappings {
		group.Mappings[i].ChannelName = names[group.Mappings[i].ChannelId]
	}
	for i := range group.Assets {
		for j := range group.Assets[i].Mappings {
			group.Assets[i].Mappings[j].ChannelName = names[group.Assets[i].Mappings[j].ChannelId]
		}
	}
}

func saveReturnedAssetMappings(userId int, target assetChannelTarget, groupMapping *model.AssetLibraryGroupChannel, assets []model.AssetLibraryAsset, upstreamAssets []upstreamAsset) error {
	for i := range assets {
		mapping := &model.AssetLibraryAssetChannel{
			AssetId:        assets[i].Id,
			GroupChannelId: groupMapping.Id,
			UserId:         userId,
			ChannelId:      target.Channel.Id,
			Status:         "Failed",
			ErrorMessage:   "upstream response did not include this asset",
		}
		if i < len(upstreamAssets) {
			upstream := upstreamAssets[i]
			mapping.UpstreamAssetId = upstream.AssetId
			mapping.AssetURL = upstream.AssetURL
			mapping.Status = upstream.Status
			mapping.ErrorCode = upstream.ErrorCode
			mapping.ErrorMessage = upstream.ErrorMessage
			if upstream.AssetName != "" || upstream.AssetType != "" {
				values := make(map[string]interface{})
				if upstream.AssetName != "" {
					values["name"] = upstream.AssetName
				}
				if upstream.AssetType != "" {
					values["asset_type"] = upstream.AssetType
				}
				_ = model.UpdateAssetLibraryAsset(userId, assets[i].Id, values)
			}
		}
		if mapping.Status == "" {
			mapping.Status = "Processing"
		}
		if err := model.SaveAssetLibraryAssetChannel(mapping); err != nil {
			return err
		}
	}
	return nil
}

func saveFailedAssetMappings(userId int, target assetChannelTarget, groupMapping *model.AssetLibraryGroupChannel, assets []model.AssetLibraryAsset, uploadErr error) error {
	for _, asset := range assets {
		mapping := &model.AssetLibraryAssetChannel{
			AssetId: asset.Id, GroupChannelId: groupMapping.Id, UserId: userId,
			ChannelId: target.Channel.Id, Status: "Failed", ErrorMessage: uploadErr.Error(),
		}
		if err := model.SaveAssetLibraryAssetChannel(mapping); err != nil {
			return err
		}
	}
	return nil
}

func updateReturnedAssetMappings(userId int, target assetChannelTarget, groupMapping *model.AssetLibraryGroupChannel, assets []model.AssetLibraryAsset, upstreamAssets []upstreamAsset) error {
	localByUpstreamId := make(map[string]model.AssetLibraryAsset)
	for _, asset := range assets {
		for _, mapping := range asset.Mappings {
			if mapping.ChannelId == target.Channel.Id && mapping.UpstreamAssetId != "" {
				localByUpstreamId[mapping.UpstreamAssetId] = asset
			}
		}
	}
	for _, upstream := range upstreamAssets {
		asset, ok := localByUpstreamId[upstream.AssetId]
		if !ok {
			continue
		}
		mapping := &model.AssetLibraryAssetChannel{
			AssetId: asset.Id, GroupChannelId: groupMapping.Id, UserId: userId,
			ChannelId: target.Channel.Id, UpstreamAssetId: upstream.AssetId,
			AssetURL: upstream.AssetURL, Status: upstream.Status,
			ErrorCode: upstream.ErrorCode, ErrorMessage: upstream.ErrorMessage,
		}
		if mapping.Status == "" {
			mapping.Status = "Processing"
		}
		if err := model.SaveAssetLibraryAssetChannel(mapping); err != nil {
			return err
		}
	}
	return nil
}

func uploadAssetFiles(ctx context.Context, target assetChannelTarget, endpointPath string, displayName string, files []*multipart.FileHeader) (*upstreamGroup, error) {
	if strings.TrimSpace(endpointPath) == "" {
		return nil, errors.New("asset upload endpoint is not configured")
	}
	requestURL, err := resolveAssetURL(target, endpointPath)
	if err != nil {
		return nil, err
	}
	key, _, keyErr := target.Channel.GetNextEnabledKey()
	if keyErr != nil {
		return nil, keyErr
	}

	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	writeErr := make(chan error, 1)
	go func() {
		defer close(writeErr)
		if displayName != "" {
			if err := multipartWriter.WriteField("displayName", displayName); err != nil {
				_ = writer.CloseWithError(err)
				writeErr <- err
				return
			}
		}
		for _, fileHeader := range files {
			file, err := fileHeader.Open()
			if err != nil {
				_ = writer.CloseWithError(err)
				writeErr <- err
				return
			}
			part, err := multipartWriter.CreateFormFile("files", fileHeader.Filename)
			if err == nil {
				_, err = io.Copy(part, file)
			}
			_ = file.Close()
			if err != nil {
				_ = writer.CloseWithError(err)
				writeErr <- err
				return
			}
		}
		if err := multipartWriter.Close(); err != nil {
			_ = writer.CloseWithError(err)
			writeErr <- err
			return
		}
		writeErr <- writer.Close()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, reader)
	if err != nil {
		_ = reader.CloseWithError(err)
		return nil, err
	}
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := GetHttpClient().Do(req)
	writerErr := <-writeErr
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if writerErr != nil {
		return nil, writerErr
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, assetLibraryResponseLimit))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, upstreamAssetError(resp.StatusCode, body)
	}
	var groupEnvelope upstreamGroupEnvelope
	if err := common.Unmarshal(body, &groupEnvelope); err == nil && groupEnvelope.Data.GroupId != "" {
		if groupEnvelope.Error != nil {
			return nil, errors.New(groupEnvelope.Error.Message)
		}
		return &groupEnvelope.Data, nil
	}
	var batchEnvelope upstreamBatchEnvelope
	if err := common.Unmarshal(body, &batchEnvelope); err != nil {
		return nil, err
	}
	if batchEnvelope.Error != nil {
		return nil, errors.New(batchEnvelope.Error.Message)
	}
	if batchEnvelope.Data.GroupId == "" {
		return nil, errors.New("upstream response is missing groupId")
	}
	return &upstreamGroup{GroupId: batchEnvelope.Data.GroupId, Assets: batchEnvelope.Data.Assets}, nil
}

func doAssetJSON(ctx context.Context, target assetChannelTarget, method string, endpointPath string, payload interface{}, output interface{}) error {
	if strings.TrimSpace(endpointPath) == "" {
		return errors.New("asset endpoint is not configured")
	}
	requestURL, err := resolveAssetURL(target, endpointPath)
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
	key, _, keyErr := target.Channel.GetNextEnabledKey()
	if keyErr != nil {
		return keyErr
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
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
		return upstreamAssetError(resp.StatusCode, responseBody)
	}
	var errorEnvelope struct {
		Error *upstreamError `json:"error"`
	}
	if err := common.Unmarshal(responseBody, &errorEnvelope); err == nil && errorEnvelope.Error != nil {
		message := strings.TrimSpace(errorEnvelope.Error.Message)
		if message == "" {
			message = strings.TrimSpace(errorEnvelope.Error.Code)
		}
		if message == "" {
			message = "upstream asset operation failed"
		}
		return errors.New(message)
	}
	if output != nil && len(responseBody) > 0 {
		if err := common.Unmarshal(responseBody, output); err != nil {
			return err
		}
	}
	return nil
}

func upstreamAssetError(status int, body []byte) error {
	var envelope struct {
		Error *upstreamError `json:"error"`
	}
	if err := common.Unmarshal(body, &envelope); err == nil && envelope.Error != nil && envelope.Error.Message != "" {
		return fmt.Errorf("upstream returned %d: %s", status, envelope.Error.Message)
	}
	message := strings.TrimSpace(string(body))
	if len(message) > 300 {
		message = message[:300]
	}
	if message == "" {
		message = http.StatusText(status)
	}
	return fmt.Errorf("upstream returned %d: %s", status, message)
}

func resolveAssetURL(target assetChannelTarget, endpointPath string) (string, error) {
	if parsed, err := url.Parse(endpointPath); err == nil && parsed.IsAbs() {
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return "", errors.New("asset endpoint must use http or https")
		}
		return parsed.String(), nil
	}
	baseURL := strings.TrimSpace(target.Config.BaseURL)
	if baseURL == "" {
		baseURL = target.Channel.GetBaseURL()
	}
	parsedBase, err := url.Parse(baseURL)
	if err != nil || parsedBase.Scheme == "" || parsedBase.Host == "" {
		return "", errors.New("asset library base URL is invalid")
	}
	if parsedBase.Scheme != "http" && parsedBase.Scheme != "https" {
		return "", errors.New("asset library base URL must use http or https")
	}
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(endpointPath, "/"), nil
}

func expandAssetPath(path string, groupId string, assetId string) string {
	path = strings.ReplaceAll(path, "{groupId}", url.PathEscape(groupId))
	path = strings.ReplaceAll(path, "{assetId}", url.PathEscape(assetId))
	return path
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
	ext := strings.ToLower(filepath.Ext(file.Filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp":
		return "Image"
	case ".mp3", ".wav", ".aac", ".m4a", ".flac", ".ogg":
		return "Audio"
	default:
		return "Video"
	}
}
