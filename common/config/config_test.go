package config

import (
	"testing"
)

func TestConfig(t *testing.T) {
	conf, err := InitConfig()
	if err != nil {
		t.Fatalf("init config: %v", err)
	}
	t.Logf("config: %+v", conf)
}
