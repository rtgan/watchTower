package mem

import (
	"context"
	"fmt"
	"log"
	"time"
	"watchTower/common/config"
	"watchTower/model"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/google/uuid"
	cli "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// MilvusMemoryStore 实现 LongTermMemory，底层使用 Milvus 向量库。
// 使用原始 Milvus client（非 eino-ext），支持 metadata 过滤表达式。
type MilvusMemoryStore struct {
	client     cli.Client
	embedder   *ark.Embedder
	collection string
	dim        int
}

// memoryCollectionFields 返回长期记忆集合的 schema 字段定义。
func memoryCollectionFields(dim int) []*entity.Field {
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
			Name:     "vector",
			DataType: entity.FieldTypeFloatVector,
			TypeParams: map[string]string{
				"dim": fmt.Sprintf("%d", dim),
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

// NewMilvusMemoryStore 创建长期记忆存储，自建 Milvus 集合。
func NewMilvusMemoryStore(ctx context.Context) (*MilvusMemoryStore, error) {
	addr := config.Conf.Milvus.Address
	if addr == "" {
		addr = "localhost:19530"
	}
	dbName := config.Conf.Milvus.DbName
	if dbName == "" {
		dbName = "agent"
	}
	collName := config.Conf.Milvus.MemoryCollectionName
	if collName == "" {
		collName = "conversation_memory"
	}

	// 1. 连接 Milvus
	agentClient, err := cli.NewClient(ctx, cli.Config{
		Address: addr,
		DBName:  dbName,
	})
	if err != nil {
		return nil, fmt.Errorf("milvus memory connect: %w", err)
	}

	// 2. 检查并创建集合
	collections, err := agentClient.ListCollections(ctx)
	if err != nil {
		agentClient.Close()
		return nil, fmt.Errorf("milvus memory list collections: %w", err)
	}

	exists := false
	for _, c := range collections {
		if c.Name == collName {
			exists = true
			break
		}
	}

	dim := 2048
	if config.Conf.DoubaoEmbeddingModel.VectorDim != "" {
		fmt.Sscanf(config.Conf.DoubaoEmbeddingModel.VectorDim, "%d", &dim)
	}

	if !exists {
		schema := &entity.Schema{
			CollectionName: collName,
			Description:    "Long-term conversation memory for semantic recall",
			Fields:         memoryCollectionFields(dim),
			EnableDynamicField: true,
		}
		if err := agentClient.CreateCollection(ctx, schema, entity.DefaultShardNumber); err != nil {
			agentClient.Close()
			return nil, fmt.Errorf("milvus memory create collection: %w", err)
		}

		// 创建索引（id, content, vector）
		for _, fieldName := range []string{"id", "content", "vector"} {
			idx, err := entity.NewIndexAUTOINDEX(entity.L2)
			if err != nil {
				agentClient.Close()
				return nil, fmt.Errorf("milvus memory create index %s: %w", fieldName, err)
			}
			if err := agentClient.CreateIndex(ctx, collName, fieldName, idx, false); err != nil {
				agentClient.Close()
				return nil, fmt.Errorf("milvus memory create index %s: %w", fieldName, err)
			}
		}

		// 加载集合到内存
		if err := agentClient.LoadCollection(ctx, collName, false); err != nil {
			agentClient.Close()
			return nil, fmt.Errorf("milvus memory load collection: %w", err)
		}
	}

	// 3. 创建 Doubao embedder
	eb := model.GetGlobalFactory().GetModelCreator(model.DoubaoEmbedderType)(ctx, config.Conf).(*model.DoubaoEmbedder).Embedder

	return &MilvusMemoryStore{
		client:     agentClient,
		embedder:   eb,
		collection: collName,
		dim:        dim,
	}, nil
}

// StoreEvicted 将淘汰消息嵌入后存入 Milvus。
func (s *MilvusMemoryStore) StoreEvicted(ctx context.Context, sessionID, userID string, evicted []*Message) error {
	if len(evicted) == 0 {
		return nil
	}

	// 构建记忆文本
	texts := make([]string, 0, len(evicted))
	ids := make([]string, 0, len(evicted))
	contents := make([]string, 0, len(evicted))
	metadatas := make([][]byte, 0, len(evicted))

	for _, msg := range evicted {
		text := fmt.Sprintf("[%s]: %s", msg.Role, msg.Content)
		// 截断过长文本
		if len(text) > 8192 {
			text = text[:8192]
		}
		texts = append(texts, text)
		id := fmt.Sprintf("%s_%s", sessionID, uuid.New().String()[:8])
		ids = append(ids, id)
		contents = append(contents, text)

		metaJSON := fmt.Sprintf(
			`{"user_id":"%s","session_id":"%s","timestamp":"%s","type":"evicted_message"}`,
			userID, sessionID, time.Now().Format(time.RFC3339),
		)
		metadatas = append(metadatas, []byte(metaJSON))
	}

	// 嵌入
	embeddings, err := s.embedStrings(ctx, texts)
	if err != nil {
		return fmt.Errorf("milvus memory embed evicted: %w", err)
	}

	// 转换为 FloatVector
	vectors := make([][]float32, len(embeddings))
	for i, emb := range embeddings {
		v32 := make([]float32, len(emb))
		for j, v := range emb {
			v32[j] = float32(v)
		}
		vectors[i] = v32
	}

	// 插入 Milvus
	idCol := entity.NewColumnVarChar("id", ids)
	contentCol := entity.NewColumnVarChar("content", contents)
	vectorCol := entity.NewColumnFloatVector("vector", s.dim, vectors)
	metaCol := entity.NewColumnJSONBytes("metadata", metadatas)

	_, err = s.client.Insert(ctx, s.collection, "", idCol, contentCol, vectorCol, metaCol)
	if err != nil {
		return fmt.Errorf("milvus memory insert: %w", err)
	}

	return nil
}

// Search 根据查询文本搜索相关长期记忆。
func (s *MilvusMemoryStore) Search(ctx context.Context, userID, query string, topK int) ([]*MemoryRecord, error) {
	if topK <= 0 {
		topK = 3
	}

	// 嵌入查询
	embeddings, err := s.embedStrings(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("milvus memory embed query: %w", err)
	}
	if len(embeddings) == 0 {
		return nil, nil
	}

	// 转换为 FloatVector
	vec := make([]float32, len(embeddings[0]))
	for i, v := range embeddings[0] {
		vec[i] = float32(v)
	}

	// 构建过滤表达式
	expr := ""
	if userID != "" {
		expr = fmt.Sprintf(`metadata["user_id"] == "%s"`, userID)
	}

	sp, err := entity.NewIndexAUTOINDEXSearchParam(1)
	if err != nil {
		return nil, fmt.Errorf("milvus memory search param: %w", err)
	}

	results, err := s.client.Search(ctx, s.collection, nil,
		expr, []string{"id", "content", "metadata"},
		[]entity.Vector{entity.FloatVector(vec)},
		"vector", entity.L2, topK, sp)
	if err != nil {
		return nil, fmt.Errorf("milvus memory search: %w", err)
	}

	// 解析结果
	records := make([]*MemoryRecord, 0)
	for _, result := range results {
		if result.Err != nil {
			log.Printf("[memory] milvus search partial error: %v", result.Err)
			continue
		}
		for i := 0; i < result.ResultCount; i++ {
			id, err := result.IDs.Get(i)
			if err != nil {
				continue
			}

			contentCol := result.Fields.GetColumn("content")
			if contentCol == nil {
				continue
			}
			content, err := contentCol.GetAsString(i)
			if err != nil {
				content = ""
			}

			metaCol := result.Fields.GetColumn("metadata")
			metaBytes := ([]byte)(nil)
			if metaCol != nil {
				if metaVal, err := metaCol.Get(i); err == nil {
					if bb, ok := metaVal.([]byte); ok {
						metaBytes = bb
					}
				}
			}

			score := float64(0)
			if i < len(result.Scores) {
				score = float64(result.Scores[i])
			}

			rec := &MemoryRecord{
				ID:      fmt.Sprintf("%v", id),
				Content: content,
				Score:   score,
			}

			// 解析 metadata 中的 user_id / session_id
			if len(metaBytes) > 0 {
				metaStr := string(metaBytes)
				rec.UserID = extractJSONField(metaStr, "user_id")
				rec.SessionID = extractJSONField(metaStr, "session_id")
			}

			records = append(records, rec)
		}
	}

	return records, nil
}

// embedStrings 调用 Doubao embedder 获取嵌入向量。
func (s *MilvusMemoryStore) embedStrings(ctx context.Context, texts []string) ([][]float64, error) {
	return s.embedder.EmbedStrings(ctx, texts)
}

// extractJSONField 从简单 JSON 字符串中提取字段值（避免引入额外依赖）。
func extractJSONField(jsonStr, key string) string {
	search := fmt.Sprintf(`"%s":"`, key)
	start := 0
	for i := 0; i < len(jsonStr)-len(search); i++ {
		if jsonStr[i:i+len(search)] == search {
			start = i + len(search)
			break
		}
	}
	if start == 0 {
		return ""
	}
	end := start
	for end < len(jsonStr) && jsonStr[end] != '"' {
		if jsonStr[end] == '\\' {
			end++
		}
		end++
	}
	return jsonStr[start:end]
}