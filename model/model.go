package model

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"time"
	"watchTower/common/config"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino/components/model"
)

// retryHTTPClient 包装 http.Client，对 5xx / 429 自动重试。
// deepseek/ark 偶发 500（服务端瞬时错误），不重试会直接 panic 中断 Agent 闭环。
func retryHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 3 * time.Minute,
		Transport: &retryTransport{
			base:      http.DefaultTransport,
			maxRetry:  4,
			backoffMs: 800,
		},
	}
}

type retryTransport struct {
	base      http.RoundTripper
	maxRetry  int
	backoffMs int
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 重试需要可重放 body：读出来缓存
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return nil, err
		}
	}
	var resp *http.Response
	var err error
	for attempt := 0; attempt <= t.maxRetry; attempt++ {
		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(bodyBytes)), nil }
		}
		resp, err = t.base.RoundTrip(req)
		if err != nil {
			return nil, err
		}
		// 5xx / 429 重试，其余直接返回
		if resp.StatusCode < 500 && resp.StatusCode != 429 {
			return resp, nil
		}
		// 读空响应体以便复用连接
		resp.Body.Close()
		if attempt == t.maxRetry {
			// 最后一次：重新发一次拿原始响应给上层（保留真实错误码）
			if bodyBytes != nil {
				req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}
			return t.base.RoundTrip(req)
		}
		time.Sleep(time.Duration(t.backoffMs<<(attempt)) * time.Millisecond)
	}
	return resp, err
}

// =================== AIModel 定义 ===================
// 一个函数类型。流式响应时，每收到 AI 吐出的一小段文字，就调用这个回调把内容实时推送给前端，用于实时展示响应内容
// type StreamCallback func(msg string)

// AIModel 定义AI模型接口
type AIModel interface {
	GetModelType() string
	// GenerateResponse(ctx context.Context, messages []*schema.Message) (*schema.Message, error)
	// StreamResponse(ctx context.Context, messages []*schema.Message, cb StreamCallback) (string, error)
}

// =================== ds_think_chat_model 实现 ===================
type DsThinkChatModel struct {
	Model model.ToolCallingChatModel
}

func NewDsThinkChatModel(ctx context.Context, conf *config.Config) *DsThinkChatModel {
	model := conf.DsThinkChatModel.Model
	apiKey := conf.DsThinkChatModel.ApiKey
	baseUrl := conf.DsThinkChatModel.BaseUrl
	cm, err := deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
		APIKey:     apiKey,
		BaseURL:    baseUrl,
		Model:      model,
		HTTPClient: retryHTTPClient(),
	})
	if err != nil {
		log.Fatalf("new deepseek chat model: %v", err)
	}
	return &DsThinkChatModel{
		Model: cm, //接口可以接收对应的实现类值或指针
	}
}

func (m *DsThinkChatModel) GetModelType() string {
	return "ds_think_chat_model"
}

// =================== ds_quick_chat_model 实现 ===================
type DsQuickChatModel struct {
	Model model.ToolCallingChatModel
}

func NewDsQuickChatModel(ctx context.Context, conf *config.Config) *DsQuickChatModel {
	model := conf.DsQuickChatModel.Model
	apiKey := conf.DsQuickChatModel.ApiKey
	baseUrl := conf.DsQuickChatModel.BaseUrl
	cm, err := deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
		APIKey:     apiKey,
		BaseURL:    baseUrl,
		Model:      model,
		HTTPClient: retryHTTPClient(),
	})
	if err != nil {
		log.Fatalf("new deepseek chat model: %v", err)
	}
	return &DsQuickChatModel{
		Model: cm, //接口可以接收对应的实现类值或指针
	}
}

func (m *DsQuickChatModel) GetModelType() string {
	return "ds_quick_chat_model"
}

// =================== doubao_embedding_model 实现 ===================
type DoubaoEmbedder struct {
	Embedder *ark.Embedder
}

func NewDoubaoEmbedder(ctx context.Context, conf *config.Config) *DoubaoEmbedder {
	model := conf.DoubaoEmbeddingModel.Model
	apiKey := conf.DoubaoEmbeddingModel.ApiKey
	apiType := ark.APITypeMultiModal //纯文本优先用 APITypeText；MultiModal 也能接收文本，但走的是多模态embedding接口
	timeout := 3 * time.Second
	retryTimes := 3
	embedder, err := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
		APIKey:     apiKey,
		Model:      model,
		APIType:    &apiType,    // 无法获取常量地址，需先赋给一个临时变量
		Timeout:    &timeout,    // 请求超时时间
		RetryTimes: &retryTimes, // 重试次数
	})
	if err != nil {
		log.Fatalf("new doubao embedder: %v", err)
	}
	return &DoubaoEmbedder{
		Embedder: embedder,
	}
}

func (m *DoubaoEmbedder) GetModelType() string {
	return "doubao_embedding_model"
}
