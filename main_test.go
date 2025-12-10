package main

import (
	"testing"
	"time"
)

// TestConfig_GetTimeoutDuration 测试超时时间解析
func TestConfig_GetTimeoutDuration(t *testing.T) {
	// 测试有效的超时时间
	config := &Config{
		Timeout: "30s",
	}
	expected := 30 * time.Second
	actual := config.GetTimeoutDuration()
	if actual != expected {
		t.Errorf("Expected %v, got %v", expected, actual)
	}

	// 测试无效的超时时间（应该返回默认值）
	config = &Config{
		Timeout: "invalid",
	}
	expected = 30 * time.Second
	actual = config.GetTimeoutDuration()
	if actual != expected {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

// TestGetDefaultFilename 测试文件名解析
func TestGetDefaultFilename(t *testing.T) {
	// 测试基本URL
	url := "https://example.com/test.pdf"
	expected := "test.pdf"
	actual := getDefaultFilename(url)
	if actual != expected {
		t.Errorf("Expected %s, got %s", expected, actual)
	}

	// 测试带参数的URL
	url = "https://example.com/test.pdf?param=value"
	expected = "test.pdf"
	actual = getDefaultFilename(url)
	if actual != expected {
		t.Errorf("Expected %s, got %s", expected, actual)
	}

	// 测试没有扩展名的URL
	url = "https://example.com/test"
	expected = "test.pdf"
	actual = getDefaultFilename(url)
	if actual != expected {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

// TestConfig_Copy 测试配置复制
func TestConfig_Copy(t *testing.T) {
	original := &Config{
		URL:        "https://example.com/test.pdf",
		OutputDir:  "/tmp",
		OutputPath: "/tmp/test.pdf",
		Timeout:    "30s",
		ChunkSize:  4 * 1024 * 1024,
		Headers: map[string]string{
			"User-Agent": "test-agent",
		},
	}

	copy := original.Copy()

	// 检查所有字段是否正确复制
	if copy.URL != original.URL {
		t.Errorf("URL not copied correctly")
	}
	if copy.OutputDir != original.OutputDir {
		t.Errorf("OutputDir not copied correctly")
	}
	if copy.OutputPath != original.OutputPath {
		t.Errorf("OutputPath not copied correctly")
	}
	if copy.Timeout != original.Timeout {
		t.Errorf("Timeout not copied correctly")
	}
	if copy.ChunkSize != original.ChunkSize {
		t.Errorf("ChunkSize not copied correctly")
	}
	if copy.Headers["User-Agent"] != original.Headers["User-Agent"] {
		t.Errorf("Headers not copied correctly")
	}
}