package knowledge_index_workflow

import (
	"context"
	mymilvus "watchTower/common/milvus"

	"github.com/cloudwego/eino/components/indexer"
)

// newIndexer component initialization function of node 'Indexer' in graph 'KnowledgeIndexing'
func newIndexer(ctx context.Context) (idr indexer.Indexer, err error) {
	return mymilvus.NewMilvusIndexer(ctx)
}
