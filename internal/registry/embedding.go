// Package registry 中的 embedding.go 提供文本向量化（Embedding）客户端，
// 用于调用 OpenAI 兼容的 Embedding API 生成文本向量，并计算向量间的余弦相似度。
// 该客户端被 Service 的语义搜索功能使用。
package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"

	"github.com/eyrihe999-stack/Skynet/internal/config"
)

// EmbeddingClient 调用外部 Embedding API 生成文本向量。
type EmbeddingClient struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

// NewEmbeddingClient 创建并返回一个新的 EmbeddingClient 实例。
func NewEmbeddingClient(cfg config.EmbeddingConfig) *EmbeddingClient {
	return &EmbeddingClient{
		baseURL: cfg.BaseURL,
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
		client:  &http.Client{},
	}
}

// Enabled 检查 embedding 是否配置（有 base_url 和 api_key 才启用）。
func (c *EmbeddingClient) Enabled() bool {
	return c.baseURL != "" && c.apiKey != ""
}

// Model 返回当前使用的 embedding 模型名称。
func (c *EmbeddingClient) Model() string { return c.model }

type embeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// Embed 生成文本的 embedding 向量。
func (c *EmbeddingClient) Embed(text string) ([]float32, error) {
	reqBody := embeddingRequest{Model: c.model, Input: text}
	body, _ := json.Marshal(reqBody)

	url := c.baseURL + "/embeddings"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse embedding response failed: %w", err)
	}

	if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("empty embedding response")
	}

	return result.Data[0].Embedding, nil
}

// CosineSimilarity 计算两个向量的余弦相似度。
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
