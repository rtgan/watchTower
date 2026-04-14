package model

import (
	"context"
	"log"
	"time"
	"watchTower/common/config"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino/components/model"
)

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
		APIKey:  apiKey,
		BaseURL: baseUrl,
		Model:   model,
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
		APIKey:  apiKey,
		BaseURL: baseUrl,
		Model:   model,
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
