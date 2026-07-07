package gemini

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// convertImagineImageRequest converts an OpenAI images request (generations or
// edits) into a Gemini generateContent request for image-capable models like
// gemini-3.1-flash-image-preview / gemini-3-pro-image-preview (nanobanana).
func convertImagineImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (*dto.GeminiChatRequest, error) {
	if strings.TrimSpace(request.Prompt) == "" {
		return nil, errors.New("prompt is required")
	}

	parts := make([]dto.GeminiPart, 0, 2)
	parts = append(parts, dto.GeminiPart{Text: request.Prompt})

	// image-to-image: collect input images from multipart form (edits endpoint)
	if info.RelayMode == constant.RelayModeImagesEdits {
		inlineImages, err := getInlineImagesFromForm(c)
		if err != nil {
			return nil, err
		}
		if len(inlineImages) == 0 {
			return nil, errors.New("image is required for image edits")
		}
		for _, img := range inlineImages {
			parts = append(parts, dto.GeminiPart{InlineData: img})
		}
	}

	geminiRequest := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{
				Role:  "user",
				Parts: parts,
			},
		},
		GenerationConfig: dto.GeminiChatGenerationConfig{
			ResponseModalities: []string{"TEXT", "IMAGE"},
		},
	}

	return geminiRequest, nil
}

// getInlineImagesFromForm reads uploaded images from the multipart form and
// converts them into Gemini inline_data parts. It accepts the standard "image"
// field as well as the "image[]" / "image[N]" array variants.
func getInlineImagesFromForm(c *gin.Context) ([]*dto.GeminiInlineData, error) {
	mf := c.Request.MultipartForm
	if mf == nil {
		if _, err := c.MultipartForm(); err != nil {
			return nil, fmt.Errorf("failed to parse image edit form request: %w", err)
		}
		mf = c.Request.MultipartForm
	}
	if mf == nil {
		return nil, errors.New("image is required for image edits")
	}

	var imageFiles []*multipart.FileHeader
	if files, ok := mf.File["image"]; ok && len(files) > 0 {
		imageFiles = append(imageFiles, files...)
	} else if files, ok := mf.File["image[]"]; ok && len(files) > 0 {
		imageFiles = append(imageFiles, files...)
	} else {
		for fieldName, files := range mf.File {
			if strings.HasPrefix(fieldName, "image[") && len(files) > 0 {
				imageFiles = append(imageFiles, files...)
			}
		}
	}

	if len(imageFiles) == 0 {
		return nil, errors.New("image is required for image edits")
	}

	inlineImages := make([]*dto.GeminiInlineData, 0, len(imageFiles))
	for _, fileHeader := range imageFiles {
		file, err := fileHeader.Open()
		if err != nil {
			return nil, errors.New("failed to open image file")
		}
		imageData, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			return nil, errors.New("failed to read image file")
		}

		mimeType := http.DetectContentType(imageData)
		inlineImages = append(inlineImages, &dto.GeminiInlineData{
			MimeType: mimeType,
			Data:     base64.StdEncoding.EncodeToString(imageData),
		})
	}

	return inlineImages, nil
}

// GeminiImageGenerationHandler handles the generateContent response of an
// image-capable Gemini model and converts it into an OpenAI images response.
// Generated images are returned as b64_json; any accompanying text output is
// preserved in the response metadata.
func GeminiImageGenerationHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	responseBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, types.NewOpenAIError(readErr, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	service.CloseResponseBodyGracefully(resp)

	var geminiResponse dto.GeminiChatResponse
	if jsonErr := common.Unmarshal(responseBody, &geminiResponse); jsonErr != nil {
		return nil, types.NewOpenAIError(jsonErr, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	if len(geminiResponse.Candidates) == 0 {
		if geminiResponse.PromptFeedback != nil && geminiResponse.PromptFeedback.BlockReason != nil {
			return nil, types.NewOpenAIError(
				errors.New("request blocked by Gemini API: "+*geminiResponse.PromptFeedback.BlockReason),
				types.ErrorCodePromptBlocked, http.StatusBadRequest)
		}
		return nil, types.NewOpenAIError(errors.New("no images generated"), types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	openAIResponse := dto.ImageResponse{
		Created: common.GetTimestamp(),
		Data:    make([]dto.ImageData, 0),
	}

	var texts []string
	for _, candidate := range geminiResponse.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.InlineData != nil && strings.HasPrefix(part.InlineData.MimeType, "image") {
				openAIResponse.Data = append(openAIResponse.Data, dto.ImageData{
					B64Json: part.InlineData.Data,
				})
			} else if part.Text != "" {
				texts = append(texts, part.Text)
			}
		}
	}

	if len(openAIResponse.Data) == 0 {
		return nil, types.NewOpenAIError(errors.New("no images generated"), types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	// preserve any accompanying text output in metadata
	if len(texts) > 0 {
		if metadata, err := common.Marshal(map[string]string{"text": strings.Join(texts, "\n")}); err == nil {
			openAIResponse.Metadata = metadata
		}
	}

	jsonResponse, jsonErr := common.Marshal(openAIResponse)
	if jsonErr != nil {
		return nil, types.NewError(jsonErr, types.ErrorCodeBadResponseBody)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = c.Writer.Write(jsonResponse)

	usage := buildUsageFromGeminiMetadata(geminiResponse.UsageMetadata, info.GetEstimatePromptTokens())
	return &usage, nil
}
