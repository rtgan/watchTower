package milvus

import (
	"context"
	"watchTower/common/config"
	"watchTower/model"

	"github.com/cloudwego/eino-ext/components/retriever/milvus"
	"github.com/cloudwego/eino/components/retriever"
)

func NewMilvusRetriever(ctx context.Context) (rtr retriever.Retriever, err error) {
	// 1. 创建milvus客户端
	cli, err := NewMilvusClient(ctx)
	if err != nil {
		return nil, err
	}
	// 2. 创建豆包embedding模型
	eb := model.GetGlobalFactory().GetModelCreator(model.DoubaoEmbedderType)(ctx, config.Conf).(*model.DoubaoEmbedder).Embedder
	// 3. 创建milvus retriever
	r, err := milvus.NewRetriever(ctx, &milvus.RetrieverConfig{
		Client:      cli,
		Collection:  config.Conf.Milvus.CollectionName,
		VectorField: "vector",
		OutputFields: []string{ //需要返回的字段
			"id",
			"content",
			"metadata",
		},
		TopK:      1,
		Embedding: eb,
	})
	if err != nil {
		return nil, err
	}
	return r, nil
}
