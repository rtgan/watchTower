package chat_workflow

import (
	"context"
	"testing"
	"watchTower/common/config"
)

func TestEmbedding(t *testing.T) {
	config.InitConfig()
	embedding, err := newEmbedding(context.Background())
	if err != nil {
		t.Fatalf("newEmbedding: %v", err)
	}
	t.Logf("embedding: %+v", embedding)
}
