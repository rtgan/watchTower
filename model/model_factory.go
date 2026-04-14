package model

import (
	"context"
	"log"
	"sync"
	"watchTower/common/config"
	"watchTower/common/enum"
)

var (
	globalFactory *AIModelFactory
	factoryOnce   sync.Once
)

// AI模型的创建工厂
type AIModelFactory struct {
	//1:ds_think_chat_model, 2:ds_quick_chat_model, 3:doubao_embedding_model
	creators map[int]ModelCreator
}

// ModelCreator 定义模型创建函数类型
type ModelCreator func(ctx context.Context, config *config.Config) AIModel

// GetGlobalFactory 获取全局单例
func GetGlobalFactory() *AIModelFactory {
	factoryOnce.Do(func() {
		globalFactory = &AIModelFactory{
			creators: make(map[int]ModelCreator),
		}
		globalFactory.registerCreators()
	})
	return globalFactory
}

// 注册模型创建函数
func (f *AIModelFactory) registerCreators() {
	f.creators[DsThinkChatModelType] = func(ctx context.Context, config *config.Config) AIModel {
		return NewDsThinkChatModel(ctx, config) //返回的是指针类型的模型
	}

	f.creators[DsQuickChatModelType] = func(ctx context.Context, config *config.Config) AIModel {
		return NewDsQuickChatModel(ctx, config) //返回的是指针类型的模型
	}

	f.creators[DoubaoEmbedderType] = func(ctx context.Context, config *config.Config) AIModel {
		return NewDoubaoEmbedder(ctx, config) //返回的是指针类型的模型
	}
}

func (f *AIModelFactory) GetModelCreator(modelType int) ModelCreator {
	if f == nil || f.creators == nil {
		f = GetGlobalFactory()
	}
	if creator, ok := f.creators[modelType]; ok {
		return creator
	}
	log.Fatalf("model creator not found: %d", modelType)
	return nil
}

const (
	DsThinkChatModelType = iota + 1
	DsQuickChatModelType
	DoubaoEmbedderType
)

var ModelTypeMap = enum.Enum{
	DsThinkChatModelType: "ds_think_chat_model",
	DsQuickChatModelType: "ds_quick_chat_model",
	DoubaoEmbedderType:   "doubao_embedding_model",
}
