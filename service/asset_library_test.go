package service

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

type assetLibraryRoundTripFunc func(*http.Request) (*http.Response, error)

func (function assetLibraryRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func assetLibraryTestResponse(request *http.Request, body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    request,
	}
}

func TestExpandAssetPath(t *testing.T) {
	path := expandAssetPath(
		"/groups/{groupId}/assets/{assetId}",
		"group with spaces",
		"asset/one",
	)
	if path != "/groups/group%20with%20spaces/assets/asset%2Fone" {
		t.Fatalf("unexpected expanded path: %s", path)
	}
}

func TestResolveAssetURL(t *testing.T) {
	channelBaseURL := "https://channel.example.com/v1"
	target := assetChannelTarget{
		Channel: &model.Channel{BaseURL: &channelBaseURL},
		Config:  &dto.AssetLibraryEndpointSettings{},
	}
	resolved, err := resolveAssetURL(target, "/api/assets")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != "https://channel.example.com/v1/api/assets" {
		t.Fatalf("unexpected relative URL: %s", resolved)
	}

	target.Config.BaseURL = "https://assets.example.com"
	resolved, err = resolveAssetURL(target, "https://override.example.com/groups")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != "https://override.example.com/groups" {
		t.Fatalf("unexpected absolute URL: %s", resolved)
	}

	if _, err := resolveAssetURL(target, "file:///tmp/assets"); err == nil {
		t.Fatal("expected non-http endpoint to be rejected")
	}
}

func TestInferAssetType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		expected    string
	}{
		{name: "photo.bin", contentType: "image/png", expected: "Image"},
		{name: "sound.wav", expected: "Audio"},
		{name: "clip.mp4", expected: "Video"},
	}
	for _, test := range tests {
		header := make(textproto.MIMEHeader)
		if test.contentType != "" {
			header.Set("Content-Type", test.contentType)
		}
		file := &multipart.FileHeader{Filename: test.name, Header: header}
		if actual := inferAssetType(file); actual != test.expected {
			t.Errorf("inferAssetType(%q) = %q, want %q", test.name, actual, test.expected)
		}
	}
}

func TestUploadAssetFiles(t *testing.T) {
	var requestBody bytes.Buffer
	requestWriter := multipart.NewWriter(&requestBody)
	part, err := requestWriter.CreateFormFile("files", "reference.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("video-content")); err != nil {
		t.Fatal(err)
	}
	if err := requestWriter.Close(); err != nil {
		t.Fatal(err)
	}

	request, err := http.NewRequest(http.MethodPost, "https://client.example.com", &requestBody)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", requestWriter.FormDataContentType())
	if err := request.ParseMultipartForm(1 << 20); err != nil {
		t.Fatal(err)
	}
	defer request.MultipartForm.RemoveAll()
	files := request.MultipartForm.File["files"]

	previousHTTPClient := httpClient
	httpClient = &http.Client{Transport: assetLibraryRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/assets/upload" {
			t.Errorf("unexpected request path: %s", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected authorization header: %s", request.Header.Get("Authorization"))
		}
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("failed to parse upstream request: %v", err)
			return assetLibraryTestResponse(request, `{}`), nil
		}
		if request.FormValue("displayName") != "Reference videos" {
			t.Errorf("unexpected displayName: %s", request.FormValue("displayName"))
		}
		file, _, err := request.FormFile("files")
		if err != nil {
			t.Errorf("missing uploaded file: %v", err)
			return assetLibraryTestResponse(request, `{}`), nil
		}
		defer file.Close()
		content, _ := io.ReadAll(file)
		if string(content) != "video-content" {
			t.Errorf("unexpected uploaded content: %q", content)
		}
		return assetLibraryTestResponse(request, `{"data":{"groupId":"group-1","assets":[{"assetId":"asset-1","assetName":"reference.mp4","assetUrl":"https://cdn.example.com/reference.mp4","assetType":"Video","status":"Active"}]},"error":null,"request_id":"req-1"}`), nil
	})}
	defer func() { httpClient = previousHTTPClient }()

	baseURL := "https://assets.example.com"
	target := assetChannelTarget{
		Channel: &model.Channel{Key: "test-key", BaseURL: &baseURL},
		Config:  &dto.AssetLibraryEndpointSettings{},
	}
	group, err := uploadAssetFiles(context.Background(), target, "/assets/upload", "Reference videos", files)
	if err != nil {
		t.Fatal(err)
	}
	if group.GroupId != "group-1" || len(group.Assets) != 1 || group.Assets[0].AssetId != "asset-1" {
		t.Fatalf("unexpected upstream group: %+v", group)
	}
}

func TestDoAssetJSONRejectsBusinessError(t *testing.T) {
	previousHTTPClient := httpClient
	httpClient = &http.Client{Transport: assetLibraryRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return assetLibraryTestResponse(request, `{"data":null,"error":{"code":"asset_failed","message":"asset operation failed"},"request_id":"req-2"}`), nil
	})}
	defer func() { httpClient = previousHTTPClient }()

	baseURL := "https://assets.example.com"
	target := assetChannelTarget{
		Channel: &model.Channel{Key: "test-key", BaseURL: &baseURL},
		Config:  &dto.AssetLibraryEndpointSettings{},
	}
	err := doAssetJSON(context.Background(), target, http.MethodGet, "/assets/group-1", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "asset operation failed") {
		t.Fatalf("expected upstream business error, got %v", err)
	}
}
