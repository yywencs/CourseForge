package cdc

import (
	"strings"
	"testing"
)

func TestLoadConfigFromEnvUsesCourseForgeDefaults(t *testing.T) {
	t.Setenv("CDC_MYSQL_ADDR", "mysql:3306")
	t.Setenv("CDC_MYSQL_USER", "canal")
	t.Setenv("CDC_MYSQL_PASSWORD", "secret")
	t.Setenv("CDC_ES_ADDR", "http://elasticsearch:9200/")
	t.Setenv("CDC_INCLUDE_TABLE_REGEX", "")
	t.Setenv("CDC_ES_INDEX_PREFIX", "")

	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadConfigFromEnv() error = %v", err)
	}
	if len(cfg.IncludeTableRegex) != 1 || cfg.IncludeTableRegex[0] != `courseforge\..*` {
		t.Fatalf("IncludeTableRegex = %#v, want courseforge default", cfg.IncludeTableRegex)
	}
	if cfg.ESIndexPrefix != "courseforge" {
		t.Fatalf("ESIndexPrefix = %q, want courseforge", cfg.ESIndexPrefix)
	}
	if cfg.ESAddr != "http://elasticsearch:9200" {
		t.Fatalf("ESAddr = %q, want trailing slash removed", cfg.ESAddr)
	}
}

func TestConfigValidateRejectsInvalidTableRegex(t *testing.T) {
	cfg := &Config{
		MySQLAddr:         "mysql:3306",
		MySQLUser:         "canal",
		MySQLServerID:     2001,
		IncludeTableRegex: []string{"["},
		ESAddr:            "http://elasticsearch:9200",
	}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "invalid regex") {
		t.Fatalf("Validate() error = %v, want invalid regex error", err)
	}
}

func TestLogicalIndexNameUsesCourseForgePrefix(t *testing.T) {
	cfg := &Config{ESIndexPrefix: "courseforge"}
	if got, want := cfg.LogicalIndexName("teaching_class"), "courseforge_teaching_class"; got != want {
		t.Fatalf("LogicalIndexName() = %q, want %q", got, want)
	}
	if got, want := cfg.LogicalIndexName("danmaku_003"), "courseforge_danmaku"; got != want {
		t.Fatalf("LogicalIndexName(shard) = %q, want %q", got, want)
	}
}
