package tika

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Client Tika 客户端
type Client struct {
	baseURL string
	timeout time.Duration
	client  *http.Client
}

// NewClient 创建新的 Tika 客户端
func NewClient(baseURL string, timeoutSeconds int) *Client {
	return &Client{
		baseURL: baseURL,
		timeout: time.Duration(timeoutSeconds) * time.Second,
		client: &http.Client{
			Timeout: time.Duration(timeoutSeconds) * time.Second,
		},
	}
}

// getContentType 根据文件扩展名获取 MIME 类型
func getContentType(filePath string) string {
	ext := filepath.Ext(filePath)
	mimeType := mime.TypeByExtension(ext)
	if mimeType != "" {
		return mimeType
	}

	// 如果无法从扩展名获取，根据常见扩展名返回
	switch ext {
	case ".pdf":
		return "application/pdf"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".doc":
		return "application/msword"
	case ".txt":
		return "text/plain"
	case ".md", ".markdown":
		return "text/markdown"
	default:
		// 不设置 Content-Type，让 Tika 自动检测
		return "application/octet-stream"
	}
}

// ExtractText 从文件路径提取文本
func (c *Client) ExtractText(filePath string) (string, error) {
	// 读取文件
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// 调用 Tika Server 的 /tika 接口提取文本
	url := fmt.Sprintf("%s/tika", c.baseURL)
	req, err := http.NewRequest("PUT", url, bytes.NewReader(fileData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// 根据文件类型设置 Content-Type，让 Tika 正确识别文件格式
	contentType := getContentType(filePath)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "text/plain")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call Tika server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Tika server returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// 读取响应文本
	text, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(text), nil
}
