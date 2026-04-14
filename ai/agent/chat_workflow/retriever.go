package chat_workflow

import (
	"context"
	mymilvus "watchTower/common/milvus"

	"github.com/cloudwego/eino/components/retriever"
)

// newRetriever component initialization function of node 'MilvusRetriever' in graph 'EinoAgent'
func newRetriever(ctx context.Context) (rtr retriever.Retriever, err error) {
	return mymilvus.NewMilvusRetriever(ctx)
}
