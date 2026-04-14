package chat_workflow

import (
	"context"
	"fmt"
	"watchTower/common/config"
	"watchTower/model"

	"github.com/cloudwego/eino/components/embedding"
)

func newEmbedding(ctx context.Context) (eb embedding.Embedder, err error) {
	creator := model.GetGlobalFactory().GetModelCreator(model.DoubaoEmbedderType)
	if creator == nil {
		return nil, fmt.Errorf("embedding model creator not found")
	}
	eb = creator(ctx, config.Conf).(*model.DoubaoEmbedder).Embedder
	return eb, nil
}
