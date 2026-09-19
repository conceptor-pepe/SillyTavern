// embedding.go 适配 OpenAI 兼容 embedding 接口，严格校验向量形状与响应大小。
package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
)

// Embedder 保存服务器端配置的独立向量服务。
type Embedder struct {
	URL, Key, ModelName string
	Client              *http.Client
}

// Model 返回索引所属空间，模型切换后旧索引不能参与余弦检索。
func (e *Embedder) Model() string { return e.ModelName }

// Embed 批量编码文本，拒绝缺失、重复索引和非有限数值。
func (e *Embedder) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	body, err := json.Marshal(struct {
		Model string   `json:"model"`
		Input []string `json:"input"`
	}{e.ModelName, texts})
	if err != nil {
		// audit:allow-no-log 错误由应用服务或调用方统一记录，仓储/供应商不重复记录内容。
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.URL, bytes.NewReader(body))
	if err != nil {
		// audit:allow-no-log 错误由应用服务或调用方统一记录，仓储/供应商不重复记录内容。
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.Key)
	client := e.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(req)
	if err != nil {
		// audit:allow-no-log 错误由应用服务或调用方统一记录，仓储/供应商不重复记录内容。
		return nil, err
	}
	return decodeVectors(response, len(texts))
}

// decodeVectors 限制响应尺寸，供应商错误内容不进入日志或用户响应。
func decodeVectors(response *http.Response, count int) (vectors [][]float64, err error) {
	defer func() { err = errors.Join(err, response.Body.Close()) }()
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("embedding request failed")
	}
	var data struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&data); err != nil {
		// audit:allow-no-log 错误由应用服务或调用方统一记录，仓储/供应商不重复记录内容。
		return nil, errors.New("invalid embedding response")
	}
	if len(data.Data) != count {
		return nil, errors.New("invalid embedding count")
	}
	vectors = make([][]float64, count)
	for _, row := range data.Data {
		if row.Index < 0 || row.Index >= count || vectors[row.Index] != nil || !validVector(row.Embedding) {
			return nil, errors.New("invalid embedding vector")
		}
		vectors[row.Index] = row.Embedding
	}
	return vectors, nil
}

// validVector 排除零向量、异常维度和非有限数值。
func validVector(vector []float64) bool {
	if len(vector) == 0 || len(vector) > 8192 {
		return false
	}
	norm := 0.0
	for _, v := range vector {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
		norm += v * v
	}
	return norm > 0 && !math.IsInf(norm, 0)
}
