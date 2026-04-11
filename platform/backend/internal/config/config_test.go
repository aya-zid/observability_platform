package config

import "testing"

func TestLoad(t *testing.T) {
	t.Setenv("REFRESH_TOKEN_PEPPER", "test-refresh-token-pepper-32-chars-minimum!!")
	cfg := Load()
	if cfg.DBName == "" {
		t.Error("DBName should not be empty")
	}
}
