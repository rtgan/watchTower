package milvus

import (
	"context"
	"fmt"
	"watchTower/common/config"

	cli "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// 创建milvus客户端
func NewMilvusClient(ctx context.Context) (cli.Client, error) {
	addr := config.Conf.Milvus.Address
	if addr == "" {
		addr = "localhost:19530"
	}
	// 1. 先连接default数据库
	defaultClient, err := cli.NewClient(ctx, cli.Config{
		Address: addr,
		DBName:  "default",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to default database: %w", err)
	}
	// 2. 检查agent数据库是否存在，不存在则创建
	databases, err := defaultClient.ListDatabases(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list databases: %w", err)
	}
	agentDBExists := false
	for _, db := range databases {
		if db.Name == config.Conf.Milvus.DbName {
			agentDBExists = true
			break
		}
	}
	if !agentDBExists {
		err = defaultClient.CreateDatabase(ctx, config.Conf.Milvus.DbName)
		if err != nil {
			return nil, fmt.Errorf("failed to create agent database: %w", err)
		}
	}

	// 3. 创建连接到agent数据库的客户端
	agentClient, err := cli.NewClient(ctx, cli.Config{
		Address: addr,
		DBName:  config.Conf.Milvus.DbName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to agent database: %w", err)
	}
	// 4. 检查biz collection是否存在，不存在则创建
	collections, err := agentClient.ListCollections(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	bizCollectionExists := false
	for _, collection := range collections {
		if collection.Name == config.Conf.Milvus.CollectionName {
			bizCollectionExists = true
			break
		}
	}

	if !bizCollectionExists {
		dim := config.Conf.DoubaoEmbeddingModel.VectorDim
		if dim == "" {
			dim = "2048"
		}
		// 创建biz collection的schema（向量维度和豆包 embedding 一致，见 etc/conf.yml doubao_embedding_model.vector_dim）
		schema := &entity.Schema{
			CollectionName: config.Conf.Milvus.CollectionName,
			Description:    "Business knowledge collection",
			Fields:         collectionSchemaFields(dim),
		}

		err = agentClient.CreateCollection(ctx, schema, entity.DefaultShardNumber)
		if err != nil {
			return nil, fmt.Errorf("failed to create biz collection: %w", err)
		}

		// 为id字段创建autoindex索引
		idIndex, err := entity.NewIndexAUTOINDEX(entity.L2)
		if err != nil {
			return nil, fmt.Errorf("failed to create id index: %w", err)
		}
		err = agentClient.CreateIndex(ctx, config.Conf.Milvus.CollectionName, "id", idIndex, false)
		if err != nil {
			return nil, fmt.Errorf("failed to create id index: %w", err)
		}

		// 为content字段创建autoindex索引
		contentIndex, err := entity.NewIndexAUTOINDEX(entity.L2)
		if err != nil {
			return nil, fmt.Errorf("failed to create content index: %w", err)
		}
		err = agentClient.CreateIndex(ctx, config.Conf.Milvus.CollectionName, "content", contentIndex, false)
		if err != nil {
			return nil, fmt.Errorf("failed to create content index: %w", err)
		}

		// 为vector字段创建autoindex索引（浮点向量用 L2；HAMMING 仅适用于二进制向量）
		vectorIndex, err := entity.NewIndexAUTOINDEX(entity.L2)
		if err != nil {
			return nil, fmt.Errorf("failed to create vector index: %w", err)
		}
		err = agentClient.CreateIndex(ctx, config.Conf.Milvus.CollectionName, "vector", vectorIndex, false)
		if err != nil {
			return nil, fmt.Errorf("failed to create vector index: %w", err)
		}
	}

	// 关闭default数据库连接
	defaultClient.Close()

	return agentClient, nil
}

func collectionSchemaFields(vectorDim string) []*entity.Field {
	return []*entity.Field{
		{
			Name:     "id",
			DataType: entity.FieldTypeVarChar,
			TypeParams: map[string]string{
				"max_length": "256",
			},
			PrimaryKey: true,
		},
		{
			Name:     "vector", // 确保匹配
			DataType: entity.FieldTypeFloatVector,
			TypeParams: map[string]string{
				"dim": vectorDim,
			},
		},
		{
			Name:     "content",
			DataType: entity.FieldTypeVarChar,
			TypeParams: map[string]string{
				"max_length": "8192",
			},
		},
		{
			Name:     "metadata",
			DataType: entity.FieldTypeJSON,
		},
	}
}
