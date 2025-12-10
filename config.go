package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const testMode = false

// Config 配置结构体
type Config struct {
	URL        string            `json:"url,omitempty"`
	OutputDir  string            `json:"output_dir"`
	OutputPath string            `json:"output_path,omitempty"`
	Timeout    string            `json:"timeout"`
	ChunkSize  int64             `json:"chunk_size"`
	Headers    map[string]string `json:"headers"`
}

// LoadConfig 加载配置文件
func LoadConfig(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("无法读取配置文件: %v", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("无法解析配置文件: %v", err)
	}

	return &config, nil
}

// SaveConfig 保存配置到文件
func SaveConfig(filePath string, config *Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("无法序列化配置: %v", err)
	}

	return os.WriteFile(filePath, data, 0644)
}

// GetTimeoutDuration 获取超时时间
func (dc *Config) GetTimeoutDuration() time.Duration {
	d, err := time.ParseDuration(dc.Timeout)
	if err != nil {
		return 30 * time.Second
	}
	return d
}
func (dc *Config) Copy() *Config {
	return &Config{
		URL:        dc.URL,
		OutputDir:  dc.OutputDir,
		OutputPath: dc.OutputPath,
		Timeout:    dc.Timeout,
		ChunkSize:  dc.ChunkSize,
		Headers:    dc.Headers,
	}
}
func getDefaultHttpHeaders() map[string]string {
	return map[string]string{
		"Origin":     "https://basic.smartedu.cn",
		"Referer":    "https://basic.smartedu.cn/",
		"Priority":   "u=1, i",
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36",
	}
}
func getDefaultConfig() *Config {
	dir, _ := os.Getwd()
	return &Config{
		OutputDir: filepath.Join(dir, "output"),
		Timeout:   "30s",
		ChunkSize: 4 * 1024 * 1024,
		Headers:   getDefaultHttpHeaders(),
	}
}
