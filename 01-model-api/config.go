package modelapi

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config 是密钥、基址和模型。
type Config struct {
	APIKey  string
	BaseURL string
	Model   string
}

// 环境变量优先，缺项从 .env 补。
func LoadConfig() (Config, error) {
	if err := loadEnvFile(); err != nil {
		return Config{}, err
	}
	cfg := Config{
		APIKey:  os.Getenv("DASHSCOPE_API_KEY"),
		BaseURL: strings.TrimRight(os.Getenv("DASHSCOPE_BASE_URL"), "/"),
		Model:   os.Getenv("DASHSCOPE_MODEL"),
	}
	if cfg.APIKey == "" || cfg.BaseURL == "" || cfg.Model == "" {
		return Config{}, fmt.Errorf("DASHSCOPE_API_KEY, DASHSCOPE_BASE_URL, and DASHSCOPE_MODEL must be set")
	}
	return cfg, nil
}

func loadEnvFile() error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	for {
		path := filepath.Join(dir, ".env")
		if _, err := os.Stat(path); err == nil {
			return parseEnvFile(path)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil
		}
		dir = parent
	}
}

func parseEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" || os.Getenv(key) != "" {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return scanner.Err()
}
