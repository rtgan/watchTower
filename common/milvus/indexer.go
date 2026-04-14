package milvus

import (
	"context"
	"fmt"
	"watchTower/common/config"
	"watchTower/model"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/indexer/milvus"
	"github.com/cloudwego/eino/schema"
)

// floatVectorRow 与 collectionSchemaFields 中的 FieldTypeFloatVector 一致。
// eino-ext 默认 DocumentConverter 将向量打成 []byte，对应其默认的 BinaryVector；
// 本项目的 vector 列为 FloatVector，插入时期望 []float32，否则会报：
// invalid type, expected []float32, got []uint8
type floatVectorRow struct {
	ID       string    `json:"id" milvus:"name:id"`
	Content  string    `json:"content" milvus:"name:content"`
	Vector   []float32 `json:"vector" milvus:"name:vector"`
	Metadata []byte    `json:"metadata" milvus:"name:metadata"`
}

// 将document转换为floatVectorRow
func documentConverterForFloatVector(_ context.Context, docs []*schema.Document, vectors [][]float64) ([]interface{}, error) {
	rows := make([]interface{}, 0, len(docs))
	for i, doc := range docs {
		meta, err := sonic.Marshal(doc.MetaData)
		if err != nil {
			return nil, fmt.Errorf("marshal metadata: %w", err)
		}
		vec := vectors[i]
		v32 := make([]float32, len(vec))
		for j, v := range vec {
			v32[j] = float32(v)
		}
		rows = append(rows, &floatVectorRow{
			ID:       doc.ID,
			Content:  doc.Content,
			Vector:   v32,
			Metadata: meta,
		})
	}
	return rows, nil
}

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
	// 4. 创建milvus indexer（自定义 DocumentConverter，与 FloatVector schema 对齐）
	indexer, err := milvus.NewIndexer(ctx, &milvus.IndexerConfig{
		Client:            cli,
		Collection:        config.Conf.Milvus.CollectionName,
		Fields:            collectionSchemaFields(dim),
		Embedding:         eb,
		DocumentConverter: documentConverterForFloatVector,
		MetricType:        milvus.L2,
	})
	if err != nil {
		return nil, err
	}
	return indexer, nil
}
