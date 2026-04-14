package milvus

import (
	"context"
	"watchTower/common/config"
	"watchTower/model"

	"github.com/cloudwego/eino-ext/components/indexer/milvus"
)

func NewMilvusIndexer(ctx context.Context) (*milvus.Indexer, error) {
	// 1. 创建milvus客户端
	cli, err := NewMilvusClient(ctx)
	if err != nil {
		return nil, err
	}
	// 2. 创建豆包embedding模型
	eb := model.GetGlobalFactory().GetModelCreator(model.DoubaoEmbedderType)(ctx, config.Conf).(*model.DoubaoEmbedder).Embedder

	// 3. 获取向量维度
	dim := config.Conf.DoubaoEmbeddingModel.VectorDim
	if dim == "" {
		dim = "2048"
	}
	// 4. 创建milvus indexer
	indexer, err := milvus.NewIndexer(ctx, &milvus.IndexerConfig{
		Client:     cli,
		Collection: config.Conf.Milvus.CollectionName,
		Fields:     collectionSchemaFields(dim),
		Embedding:  eb,
	})
	if err != nil {
		return nil, err
	}
	return indexer, nil
}
