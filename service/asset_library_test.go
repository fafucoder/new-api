package service

import (
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
)

func writeTempFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

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

func TestBuildVolcActionURL(t *testing.T) {
	upstream := &model.AssetLibraryUpstream{
		BaseURL: "https://assets.example.com/api/asset-library",
	}
	resolved, err := buildVolcActionURL(upstream, "CreateAssetGroup")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != "https://assets.example.com/api/asset-library?Action=CreateAssetGroup&Version=2024-01-01" {
		t.Fatalf("unexpected action URL: %s", resolved)
	}

	upstream.Version = "2025-06-01"
	resolved, err = buildVolcActionURL(upstream, "GetAsset")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != "https://assets.example.com/api/asset-library?Action=GetAsset&Version=2025-06-01" {
		t.Fatalf("unexpected action URL with version override: %s", resolved)
	}

	upstream.BaseURL = "file:///tmp/assets"
	if _, err := buildVolcActionURL(upstream, "GetAsset"); err == nil {
		t.Fatal("expected non-http base URL to be rejected")
	}
}

func TestBuildOpenAIURL(t *testing.T) {
	upstream := &model.AssetLibraryUpstream{BaseURL: "https://api.openai.com/v1"}
	resolved, err := buildOpenAIURL(upstream, "files")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != "https://api.openai.com/v1/files" {
		t.Fatalf("unexpected OpenAI URL: %s", resolved)
	}
	resolved, err = buildOpenAIURL(upstream, "files/file-123")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != "https://api.openai.com/v1/files/file-123" {
		t.Fatalf("unexpected OpenAI file URL: %s", resolved)
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
		{name: "art.heic", expected: "Image"},
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

func TestCreateVolcGroup(t *testing.T) {
	previousHTTPClient := httpClient
	httpClient = &http.Client{Transport: assetLibraryRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Query().Get("Action") != "CreateAssetGroup" {
			t.Errorf("unexpected Action: %s", request.URL.Query().Get("Action"))
		}
		if request.URL.Query().Get("Version") != "2024-01-01" {
			t.Errorf("unexpected Version: %s", request.URL.Query().Get("Version"))
		}
		if request.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected authorization header: %s", request.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(request.Body)
		if !strings.Contains(string(body), `"Name":"wuxing-test"`) {
			t.Errorf("unexpected request body: %s", body)
		}
		if !strings.Contains(string(body), `"GroupType":"AIGC"`) {
			t.Errorf("expected GroupType AIGC, got: %s", body)
		}
		return assetLibraryTestResponse(request, `{"ResponseMetadata":{"RequestId":"req-1","Action":"CreateAssetGroup"},"Result":{"Id":"group-na-123"}}`), nil
	})}
	defer func() { httpClient = previousHTTPClient }()

	upstream := &model.AssetLibraryUpstream{
		Format: model.AssetLibraryFormatVolcengine, APIKey: "test-key",
		BaseURL: "https://assets.example.com/api/asset-library",
	}
	groupId, err := createVolcGroup(context.Background(), upstream, "wuxing-test", "AIGC")
	if err != nil {
		t.Fatal(err)
	}
	if groupId != "group-na-123" {
		t.Fatalf("unexpected group id: %s", groupId)
	}
}

func TestCreateVolcAsset(t *testing.T) {
	previousHTTPClient := httpClient
	httpClient = &http.Client{Transport: assetLibraryRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Query().Get("Action") != "CreateAsset" {
			t.Errorf("unexpected Action: %s", request.URL.Query().Get("Action"))
		}
		body, _ := io.ReadAll(request.Body)
		if !strings.Contains(string(body), `"GroupId":"group-na-123"`) {
			t.Errorf("missing GroupId in body: %s", body)
		}
		if !strings.Contains(string(body), `"URL":"https://cdn.example.com/logo.png"`) {
			t.Errorf("missing URL in body: %s", body)
		}
		if !strings.Contains(string(body), `"AssetType":"Image"`) {
			t.Errorf("missing AssetType in body: %s", body)
		}
		return assetLibraryTestResponse(request, `{"ResponseMetadata":{"RequestId":"req-2"},"Result":{"Id":"asset-na-456"}}`), nil
	})}
	defer func() { httpClient = previousHTTPClient }()

	upstream := &model.AssetLibraryUpstream{
		Format: model.AssetLibraryFormatVolcengine, APIKey: "test-key",
		BaseURL: "https://assets.example.com/api/asset-library",
	}
	item := storedAssetFile{name: "logo.png", assetType: "Image", publicURL: "https://cdn.example.com/logo.png"}
	result, err := createVolcAsset(context.Background(), upstream, "group-na-123", item)
	if err != nil {
		t.Fatal(err)
	}
	if result.UpstreamAssetId != "asset-na-456" || result.Status != "Processing" {
		t.Fatalf("unexpected asset result: %+v", result)
	}
}

func TestGetVolcAsset(t *testing.T) {
	previousHTTPClient := httpClient
	httpClient = &http.Client{Transport: assetLibraryRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Query().Get("Action") != "GetAsset" {
			t.Errorf("unexpected Action: %s", request.URL.Query().Get("Action"))
		}
		return assetLibraryTestResponse(request, `{"ResponseMetadata":{},"Result":{"Id":"asset-na-456","Status":"Active","URL":"https://cdn.example.com/logo.png"}}`), nil
	})}
	defer func() { httpClient = previousHTTPClient }()

	upstream := &model.AssetLibraryUpstream{
		Format: model.AssetLibraryFormatVolcengine, APIKey: "test-key",
		BaseURL: "https://assets.example.com/api/asset-library",
	}
	detail, err := getVolcAsset(context.Background(), upstream, "asset-na-456")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Status != "Active" || detail.AssetURL != "https://cdn.example.com/logo.png" {
		t.Fatalf("unexpected asset detail: %+v", detail)
	}
}

func TestDoVolcActionRejectsBusinessError(t *testing.T) {
	previousHTTPClient := httpClient
	httpClient = &http.Client{Transport: assetLibraryRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return assetLibraryTestResponse(request, `{"ResponseMetadata":{"RequestId":"req-3","Error":{"Code":"AssetFailed","Message":"asset operation failed"}},"Result":null}`), nil
	})}
	defer func() { httpClient = previousHTTPClient }()

	upstream := &model.AssetLibraryUpstream{
		Format: model.AssetLibraryFormatVolcengine, APIKey: "test-key",
		BaseURL: "https://assets.example.com/api/asset-library",
	}
	err := doVolcAction(context.Background(), upstream, "GetAsset", map[string]interface{}{"Id": "asset-1"}, nil)
	if err == nil || !strings.Contains(err.Error(), "asset operation failed") {
		t.Fatalf("expected upstream business error, got %v", err)
	}
}

func TestCreateOpenAIFile(t *testing.T) {
	previousHTTPClient := httpClient
	httpClient = &http.Client{Transport: assetLibraryRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/v1/files" {
			t.Errorf("unexpected path: %s", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("unexpected authorization: %s", request.Header.Get("Authorization"))
		}
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("failed to parse multipart: %v", err)
			return assetLibraryTestResponse(request, `{}`), nil
		}
		if request.FormValue("purpose") != "user_data" {
			t.Errorf("unexpected purpose: %s", request.FormValue("purpose"))
		}
		file, _, err := request.FormFile("file")
		if err != nil {
			t.Errorf("missing uploaded file: %v", err)
			return assetLibraryTestResponse(request, `{}`), nil
		}
		defer file.Close()
		content, _ := io.ReadAll(file)
		if string(content) != "image-bytes" {
			t.Errorf("unexpected content: %q", content)
		}
		return assetLibraryTestResponse(request, `{"id":"file-abc","object":"file","bytes":11,"filename":"logo.png","purpose":"user_data","status":"processed"}`), nil
	})}
	defer func() { httpClient = previousHTTPClient }()

	dir := t.TempDir()
	localPath := dir + "/logo.png"
	if err := writeTempFile(localPath, "image-bytes"); err != nil {
		t.Fatal(err)
	}
	upstream := &model.AssetLibraryUpstream{
		Format: model.AssetLibraryFormatOpenAI, APIKey: "sk-test",
		BaseURL: "https://api.openai.com/v1",
	}
	item := storedAssetFile{name: "logo.png", assetType: "Image", localPath: localPath}
	result, err := createOpenAIFile(context.Background(), upstream, item)
	if err != nil {
		t.Fatal(err)
	}
	if result.UpstreamAssetId != "file-abc" || result.Status != "Active" {
		t.Fatalf("unexpected openai result: %+v", result)
	}
}
