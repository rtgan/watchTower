package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"watchTower/ai/agent/knowledge_index_workflow"
	"watchTower/common/config"
	"watchTower/common/fileloader"
	logcallback "watchTower/common/log_callback"

	"watchTower/common/milvus"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/compose"
)

func main() {
	config.InitConfig()
	reset := flag.Bool("reset", false, "先 drop Milvus 集合再重建（清空旧/污染数据，如曾把工程文档误索引过）")
	flag.Parse()

	ctx := context.Background()

	// --reset：drop 集合，确保重建后只剩当前 KB 目录的文档（不含历史污染记录）
	if *reset {
		cli, err := milvus.NewMilvusClient(ctx)
		if err != nil {
			panic(err)
		}
		coll := config.Conf.Milvus.CollectionName
		_ = cli.ReleaseCollection(ctx, coll) // 已 load 需先 release，忽略错误
		if err := cli.DropCollection(ctx, coll); err != nil {
			fmt.Printf("[warn] drop collection %s: %v\n", coll, err)
		} else {
			fmt.Printf("[info] dropped collection %s, will recreate\n", coll)
		}
	}

	r, err := knowledge_index_workflow.BuildKnowledgeIndexing(ctx)
	if err != nil {
		panic(err)
	}

	// 遍历 config.FileDir（KB 目录，仅业务文档；工程文档在 docs/ 不进 Milvus）
	kbDir := config.Conf.FileDir
	if kbDir == "" {
		kbDir = "./knowledge"
	}
	err = filepath.WalkDir(kbDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk dir failed: %w", err)
		}
		if d.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ".md") {
			fmt.Printf("[skip] not a markdown file: %s\n", path)
			return nil
		}

		fmt.Printf("[start] indexing file: %s\n", path)
		// 删除biz数据metadata中_source一样的数据，避免重复
		loader, err := fileloader.NewFileLoader(ctx)
		if err != nil {
			return err
		}
		docs, err := loader.Load(ctx, document.Source{URI: path})
		if err != nil {
			return err
		}
		cli, err := milvus.NewMilvusClient(ctx)
		if err != nil {
			return err
		}
		// 查询所有metadata中_source一样的数据并删除
		expr := fmt.Sprintf(`metadata["_source"] == "%s"`, docs[0].MetaData["_source"])
		queryResult, err := cli.Query(ctx, config.Conf.Milvus.CollectionName, []string{}, expr, []string{"id"})
		if err != nil {
			return err
		} else if len(queryResult) > 0 {
			// 提取所有需要删除的id
			var idsToDelete []string
			for _, column := range queryResult {
				if column.Name() == "id" {
					for i := 0; i < column.Len(); i++ {
						id, err := column.GetAsString(i)
						if err == nil {
							idsToDelete = append(idsToDelete, id)
						}
					}
				}
			}
			// 删除这些数据
			if len(idsToDelete) > 0 {
				deleteExpr := fmt.Sprintf(`id in ["%s"]`, strings.Join(idsToDelete, `","`))
				err = cli.Delete(ctx, config.Conf.Milvus.CollectionName, "", deleteExpr)
				if err != nil {
					fmt.Printf("[warn] delete existing data failed: %v\n", err)
				} else {
					fmt.Printf("[info] deleted %d existing records with _source: %s\n", len(idsToDelete), docs[0].MetaData["_source"])
				}
			}
		}

		// 走eino-graph：取文档并切片+向量化+存入Milvus(ids：写入 Milvus 的每一条记录的id)----id在MarkdownSplitter节点的newDocumentTransformer方法中生成
		ids, err := r.Invoke(ctx, document.Source{URI: path}, compose.WithCallbacks(logcallback.LogCallback(&config.Conf.LogCallback)))
		if err != nil {
			return fmt.Errorf("invoke index graph failed: %w", err)
		}
		fmt.Printf("[done] indexing file: %s, len of parts: %d，%s\n", path, len(ids), ids)
		return nil
	})
	if err != nil {
		fmt.Printf("[error] index walk: %v\n", err)
	}

	// 刷新 Milvus 搜索视图：reindex（删/改/重建）后，已加载的搜索快照会滞后，仍返回已删记录。
	// release+load 强制重新加载，确保 query_internal_docs 检索到最新数据。
	rc, rerr := milvus.NewMilvusClient(ctx)
	if rerr != nil {
		fmt.Printf("[warn] reload client: %v\n", rerr)
	} else {
		coll := config.Conf.Milvus.CollectionName
		_ = rc.ReleaseCollection(ctx, coll)
		if e := rc.LoadCollection(ctx, coll, false); e != nil {
			fmt.Printf("[warn] reload collection: %v\n", e)
		} else {
			fmt.Printf("[info] reloaded collection %s for fresh search\n", coll)
		}
	}
}
