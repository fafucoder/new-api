package controller

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	logoUploadDir   = "./upload"
	logoURLPrefix   = "/upload/"
	logoMaxSize     = 10 * 1024 * 1024
	logoSniffLength = 512
)

var logoAllowedExts = map[string]struct{}{
	".png":  {},
	".jpg":  {},
	".jpeg": {},
	".gif":  {},
	".webp": {},
	".svg":  {},
	".ico":  {},
}

func UploadLogo(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		common.ApiErrorMsg(c, "未接收到上传的文件")
		return
	}

	if fileHeader.Size <= 0 {
		common.ApiErrorMsg(c, "上传的文件为空")
		return
	}
	if fileHeader.Size > logoMaxSize {
		common.ApiErrorMsg(c, "文件大小超过 10MB 限制")
		return
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if _, ok := logoAllowedExts[ext]; !ok {
		common.ApiErrorMsg(c, "不支持的图片格式，仅支持 png/jpg/jpeg/gif/webp/svg/ico")
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		common.ApiErrorMsg(c, "打开上传文件失败")
		return
	}
	defer file.Close()

	sniffBuf := make([]byte, logoSniffLength)
	n, err := file.Read(sniffBuf)
	if err != nil && n == 0 {
		common.ApiErrorMsg(c, "读取上传文件失败")
		return
	}
	contentType := http.DetectContentType(sniffBuf[:n])
	// SVG 经常被嗅探为 text/xml；仅当扩展名是 .svg 时放行 xml 类型。
	isImage := strings.HasPrefix(contentType, "image/")
	isSvgXML := ext == ".svg" && (strings.Contains(contentType, "xml") || strings.Contains(contentType, "text/plain"))
	if !isImage && !isSvgXML {
		common.ApiErrorMsg(c, "文件不是有效的图片")
		return
	}

	if err := os.MkdirAll(logoUploadDir, 0755); err != nil {
		common.ApiErrorMsg(c, "创建上传目录失败")
		return
	}

	filename := "logo-" + uuid.New().String() + ext
	destPath := filepath.Join(logoUploadDir, filename)
	if err := c.SaveUploadedFile(fileHeader, destPath); err != nil {
		common.ApiErrorMsg(c, "保存文件失败: "+err.Error())
		return
	}

	common.ApiSuccess(c, logoURLPrefix+filename)
}
