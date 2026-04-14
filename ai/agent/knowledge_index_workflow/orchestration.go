package knowledge_index_workflow

import (
	"context"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/compose"
)

// 生成可运行的EinoGraph(Agent图)
func BuildKnowledgeIndexing(ctx context.Context) (r compose.Runnable[document.Source, []string], err error) {
	const (
		FileLoader       = "FileLoader"
		MarkdownSplitter = "MarkdownSplitter"
		Indexer          = "Indexer"
	)
	g := compose.NewGraph[document.Source, []string]()
	fileLoaderKeyOfLoader, err := newLoader(ctx)
	if err != nil {
		return nil, err
	}
	_ = g.AddLoaderNode(FileLoader, fileLoaderKeyOfLoader)
	markdownSplitterKeyOfDocumentTransformer, err := newDocumentTransformer(ctx)
	if err != nil {
		return nil, err
	}
	_ = g.AddDocumentTransformerNode(MarkdownSplitter, markdownSplitterKeyOfDocumentTransformer)
	indexerKeyOfIndexer, err := newIndexer(ctx) //该节点会自动完成「向量化 + 写入 Milvus」这类工作
	if err != nil {
		return nil, err
	}
	_ = g.AddIndexerNode(Indexer, indexerKeyOfIndexer)
	_ = g.AddEdge(compose.START, FileLoader)
	_ = g.AddEdge(Indexer, compose.END)
	_ = g.AddEdge(FileLoader, MarkdownSplitter)
	_ = g.AddEdge(MarkdownSplitter, Indexer)
	r, err = g.Compile(ctx, compose.WithGraphName("KnowledgeIndexing"), compose.WithNodeTriggerMode(compose.AnyPredecessor))
	if err != nil {
		return nil, err
	}
	return r, err
}
