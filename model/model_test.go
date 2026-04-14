package model

import (
	"context"
	"testing"
	"watchTower/common/config"
)

func TestDsThinkChatModel(t *testing.T) {
	config.InitConfig()
	dsThinkChatModel := NewDsThinkChatModel(context.Background(), config.Conf)
	t.Logf("dsThinkChatModel: %+v", dsThinkChatModel)
}

func TestDsQuickChatModel(t *testing.T) {
	config.InitConfig()
	dsQuickChatModel := NewDsQuickChatModel(context.Background(), config.Conf)
	t.Logf("dsQuickChatModel: %+v", dsQuickChatModel)
}

func TestDoubaoEmbedder(t *testing.T) {
	config.InitConfig()
	doubaoEmbedder := NewDoubaoEmbedder(context.Background(), config.Conf)
	t.Logf("doubaoEmbedder: %+v", doubaoEmbedder)
}

func TestModelFactory(t *testing.T) {
	config.InitConfig()
	modelFactory := GetGlobalFactory()
	t.Logf("modelFactory: %+v", modelFactory)
	dsThinkChatModel := modelFactory.creators[DsThinkChatModelType](context.Background(), config.Conf)
	t.Logf("dsThinkChatModel: %+v", dsThinkChatModel)
	dsQuickChatModel := modelFactory.creators[DsQuickChatModelType](context.Background(), config.Conf)
	t.Logf("dsQuickChatModel: %+v", dsQuickChatModel)
	doubaoEmbedder := modelFactory.creators[DoubaoEmbedderType](context.Background(), config.Conf)
	t.Logf("doubaoEmbedder: %+v", doubaoEmbedder)
}
