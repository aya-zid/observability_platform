package config

import "testing"

func TestLoad(t *testing.T) {
	cfg := Load()
	if cfg.DBName == "" {
		t.Error("DBName should not be empty")
	}
}