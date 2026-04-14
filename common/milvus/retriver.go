package milvus

import (
	"context"
	"watchTower/common/config"
	"watchTower/model"

	"github.com/cloudwego/eino-ext/components/retriever/milvus"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// 与 FieldTypeFloatVector 集合一致。eino-ext 默认把查询向量转成 BinaryVector，会触发
// vector type must be the same ... VECTOR_FLOAT vs VECTOR_BINARY
func floatVectorConverter(_ context.Context, vectors [][]float64) ([]entity.Vector, error) {
	out := make([]entity.Vector, 0, len(vectors))
	for _, v := range vectors {
		f32 := make([]float32, len(v))
		for i, x := range v {
			f32[i] = float32(x)
		}
		out = append(out, entity.FloatVector(f32))
	}
	return out, nil
}

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
		Client:          cli,
		Collection:      config.Conf.Milvus.CollectionName,
		VectorField:     "vector",
		VectorConverter: floatVectorConverter,
		MetricType:      entity.L2,
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
