package chat

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"watchTower/ai/agent/knowledge_index_workflow"
	"watchTower/common/config"
	fileloader "watchTower/common/fileLoader"
	logcallback "watchTower/common/log_callback"
	"watchTower/common/milvus"
	"watchTower/model/vo"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/compose"
	"github.com/gin-gonic/gin"
)

func FileUpload(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 1*time.Minute)
	defer cancel()

	file, err := c.FormFile("file")
	if err != nil || file == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errors.New("file upload error")})
		return
	}
	filePath := filepath.Join(config.Conf.FileDir, file.Filename)
	if err := c.SaveUploadedFile(file, filePath); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errors.New("file save error")})
		return
	}
	err = buildIntoIndex(ctx, filePath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errors.New("build into index error")})
		return
	}
	res := &vo.FileUploadRes{
		FileName: file.Filename,
		FilePath: filePath,
		FileSize: file.Size,
	}
	c.JSON(http.StatusOK, res)
}

func buildIntoIndex(ctx context.Context, path string) error {
	r, err := knowledge_index_workflow.BuildKnowledgeIndexing(ctx)
	if err != nil {
		return err
	}

	if !strings.HasSuffix(path, ".md") {
		fmt.Printf("[skip] not a markdown file: %s\n", path)
		return nil
	}

	fmt.Printf("[start] indexing file: %s\n", path)
	// 删除biz数据metadata中_source一样的数据
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

	// 走eino-graph：取文档并切片+向量化+存入Milvus(ids：写入 Milvus 的每一条记录的id)————id在MarkdownSplitter节点的newDocumentTransformer方法中生成
	ids, err := r.Invoke(ctx, document.Source{URI: path}, compose.WithCallbacks(logcallback.LogCallback(&config.Conf.LogCallback)))
	if err != nil {
		return fmt.Errorf("invoke index graph failed: %w", err)
	}
	fmt.Printf("[done] indexing file: %s, len of parts: %d，%s\n", path, len(ids), ids)
	return nil
}
