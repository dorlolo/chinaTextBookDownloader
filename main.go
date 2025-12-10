package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func main() {
	// 添加-mode参数来选择运行模式
	mode := flag.String("mode", "cli", "运行模式: cli(命令行模式) 或 web(Web界面模式)")
	webPort := flag.String("port", "8080", "Web服务端口(仅在-web模式下有效)")
	configPath := flag.String("config", "config.json", "配置文件路径")

	// 原有的命令行参数
	var cliConfig Config
	flag.StringVar(&cliConfig.URL, "url", "", "PDF 文件的 HTTP/HTTPS URL（必填，仅CLI模式）")
	flag.StringVar(&cliConfig.OutputPath, "out", "", "输出文件路径（可选，默认当前目录下的原文件名，仅CLI模式）")
	flag.StringVar(&cliConfig.Timeout, "timeout", "30s", "下载超时时间（如 1m 表示1分钟，仅CLI模式）")
	flag.Int64Var(&cliConfig.ChunkSize, "chunk", 4*1024*1024, "分块下载大小（默认4MB，仅CLI模式）")

	// 新增的请求头参数
	var headers headerFlags
	flag.Var(&headers, "H", "HTTP请求头，格式: Key:Value（可多次使用，仅CLI模式）")

	flag.Parse()

	// 根据模式执行不同逻辑
	switch *mode {
	case "web":
		// Web模式
		runWebMode(*configPath, *webPort)
	case "cli":
		fallthrough
	default:
		// CLI模式（默认）
		runCLIMode(*configPath, &cliConfig, headers)
	}
}

// headerFlags 实现flag.Value接口，用于处理多个-H参数
type headerFlags map[string]string

func (h *headerFlags) String() string {
	return fmt.Sprintf("%v", *h)
}

func (h *headerFlags) Set(value string) error {
	if *h == nil {
		*h = make(map[string]string)
	}
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("请求头格式错误: %s，应为 Key:Value", value)
	}
	key := strings.TrimSpace(parts[0])
	val := strings.TrimSpace(parts[1])
	(*h)[key] = val
	return nil
}

// runWebMode 运行Web界面模式
func runWebMode(configPath, port string) {
	// 尝试加载现有配置文件
	config, err := LoadConfig(configPath)
	if err != nil {
		fmt.Printf("警告: 无法加载配置文件: %v，将使用默认配置\n", err)
		// 创建默认配置
		config = getDefaultConfig()
		// 保存默认配置到文件
		if saveErr := SaveConfig(configPath, config); saveErr != nil {
			fmt.Printf("警告: 无法保存默认配置文件: %v\n", saveErr)
		}
	}

	// 启动Web服务器
	server := NewWebServer(config, configPath)
	if err := server.Start(port); err != nil {
		fmt.Printf("Web服务器启动失败: %v\n", err)
		os.Exit(1)
	}
}

// runCLIMode 运行命令行模式
func runCLIMode(configPath string, cliConfig *Config, headers headerFlags) {
	var config *Config

	// 尝试加载配置文件
	if _, err := os.Stat(configPath); err == nil {
		// 配置文件存在，加载它
		var loadErr error
		config, loadErr = LoadConfig(configPath)
		if loadErr != nil {
			fmt.Printf("警告: 无法加载配置文件: %v\n", loadErr)
		}
	}

	// 如果没有配置文件或者加载失败，创建默认配置
	if config == nil {
		config = getDefaultConfig()
	} else {
		timeOut := config.GetTimeoutDuration()
		// 使用配置文件中的值覆盖命令行参数（如果提供了的话）
		if cliConfig.URL != "" {
			config.URL = cliConfig.URL
		}
		if cliConfig.OutputPath != "" {
			config.OutputPath = cliConfig.OutputPath
		}
		// 注意：对于time.Duration类型的参数，我们只在不是默认值时才覆盖
		if timeOut != 30*time.Second {
			config.Timeout = cliConfig.Timeout
		}
		if cliConfig.ChunkSize != 4*1024*1024 { // 不是默认值
			config.ChunkSize = cliConfig.ChunkSize
		}
	}

	// 验证必要参数
	if config.URL == "" {
		fmt.Println("错误: 必须提供PDF文件URL")
		fmt.Println("使用方法:")
		fmt.Println("  命令行模式: go run main.go -url=\"PDF文件URL\" [其他选项]")
		fmt.Println("  Web模式: go run main.go -mode=web [-port=端口号]")
		os.Exit(1)
	}

	// 设置默认输出路径
	if config.OutputPath == "" {
		config.OutputPath = filepath.Join(config.OutputDir, getDefaultFilename(config.URL))
	}

	// 创建下载配置
	downloadConfig := cliConfig.Copy()

	// 创建上下文
	ctx, cancel := context.WithTimeout(context.Background(), downloadConfig.GetTimeoutDuration())
	defer cancel()

	// 执行下载
	err := downloadPDF(ctx, *downloadConfig)
	if err != nil {
		fmt.Printf("下载失败：%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n下载完成！文件保存至：%s\n", config.OutputPath)
}

// 下载 PDF 文件（支持断点续传）
func downloadPDF(ctx context.Context, config Config) error {
	return downloadPDFWithProgress(ctx, config, nil)
}

// 下载 PDF 文件（支持断点续传）带进度回调
func downloadPDFWithProgress(ctx context.Context, config Config, progressCallback func(percent float64, downloaded, total int64)) error {
	// 检查文件是否已存在（支持断点续传）
	var startPos int64 = 0
	outputFile, err := os.OpenFile(config.OutputPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		fmt.Println("错误:", err)
		return fmt.Errorf("无法创建文件：%v", err)
	}
	defer outputFile.Close()

	// 获取已下载的文件大小（用于断点续传）
	fileInfo, err := outputFile.Stat()
	if err == nil && fileInfo.Size() > 0 {
		startPos = fileInfo.Size()
		fmt.Printf("发现已下载 %d bytes，将继续下载...\n", startPos)
	}

	// 创建 HTTP 请求
	var totalSize int64
	var resp *http.Response
	if testMode {
		totalSize = 1024 * 1024 * 1024
		resp = &http.Response{
			StatusCode:    200,
			Body:          io.NopCloser(bytes.NewReader(make([]byte, totalSize))),
			ContentLength: totalSize,
			Header:        http.Header{},
		}
	} else {
		req, err := http.NewRequestWithContext(ctx, "GET", config.URL, nil)
		if err != nil {
			return fmt.Errorf("创建请求失败：%v", err)
		}

		// 设置请求头
		if config.Headers != nil {
			for k, v := range config.Headers {
				req.Header.Set(k, v)
			}
		}
		// 添加默认请求头
		defaultHeaders := getDefaultHttpHeaders()
		for k, v := range defaultHeaders {
			if req.Header.Get(k) == "" {
				req.Header.Set(k, v)
			}
		}
		// 设置 Range 请求头（断点续传）
		if startPos > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", startPos))
		}

		// 发送请求
		client := &http.Client{
			Transport: &http.Transport{
				// 禁用 HTTP/2（部分服务器兼容性问题）
				ForceAttemptHTTP2: false,
			},
		}
		resp, err = client.Do(req)
		if err != nil {
			return fmt.Errorf("请求失败：%v", err)
		}
		defer resp.Body.Close()

		// 检查响应状态码
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("服务器返回错误状态码：%d (%s)", resp.StatusCode, resp.Status)
		}

		// 获取文件总大小
		totalSize, err = getTotalFileSize(resp, startPos)
		if err != nil {
			return fmt.Errorf("获取文件大小失败：%v", err)
		}
		// 移动文件指针到已下载位置的末尾
		if _, err := outputFile.Seek(startPos, io.SeekStart); err != nil {
			return fmt.Errorf("移动文件指针失败：%v", err)
		}
	}
	// 下载并写入文件
	buffer := make([]byte, config.ChunkSize)
	downloadedSize := startPos
	progressTicker := time.NewTicker(200 * time.Millisecond) // 进度更新频率
	defer progressTicker.Stop()

	fmt.Printf("开始下载（总大小：%.2f MB）...\n", float64(totalSize)/1024/1024)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("下载超时或被取消：%v", ctx.Err())
		default:
			// 读取数据
			n, err := resp.Body.Read(buffer)
			if n > 0 {
				// 写入文件
				if _, writeErr := outputFile.Write(buffer[:n]); writeErr != nil {
					return fmt.Errorf("写入文件失败：%v", writeErr)
				}
				downloadedSize += int64(n)

				// 显示进度（定期更新）
				select {
				case <-progressTicker.C:
					// 从输出路径中提取文件名
					filename := filepath.Base(config.OutputPath)
					printProgress(filename, downloadedSize, totalSize)

					// 调用进度回调函数（如果提供）
					if progressCallback != nil {
						percent := float64(downloadedSize) / float64(totalSize) * 100
						progressCallback(percent, downloadedSize, totalSize)
					}
				default:
				}
			}

			// 检查是否下载完成
			if err == io.EOF {
				// 最后更新一次进度
				filename := filepath.Base(config.OutputPath)
				printProgress(filename, downloadedSize, totalSize)

				// 调用进度回调函数（如果提供）
				if progressCallback != nil {
					percent := float64(downloadedSize) / float64(totalSize) * 100
					progressCallback(percent, downloadedSize, totalSize)
				}

				return nil
			} else if err != nil {
				return fmt.Errorf("读取数据失败：%v", err)
			}
		}
	}
}

// 从响应头获取文件总大小
func getTotalFileSize(resp *http.Response, startPos int64) (int64, error) {
	// 处理 206 Partial Content（断点续传）
	if resp.StatusCode == http.StatusPartialContent {
		contentRange := resp.Header.Get("Content-Range")
		if contentRange == "" {
			return 0, fmt.Errorf("服务器不支持断点续传（缺少 Content-Range 头）")
		}
		// Content-Range 格式：bytes 0-1023/4096 或 bytes 1024-/4096
		parts := strings.Split(contentRange, "/")
		if len(parts) != 2 {
			return 0, fmt.Errorf("无效的 Content-Range 格式：%s", contentRange)
		}
		totalSize, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("解析文件大小失败：%v", err)
		}
		return totalSize, nil
	}

	// 处理 200 OK（完整下载）
	contentLength := resp.ContentLength
	if contentLength <= 0 {
		return 0, fmt.Errorf("服务器未返回文件大小（Content-Length 为空）")
	}
	return contentLength + startPos, nil
}

// printProgress 打印下载进度
func printProgress(name string, downloaded, total int64) {
	if total <= 0 {
		return
	}

	// 如果文件名超过10个字符，中间用省略号替换
	displayName := name
	if len(name) > 10 {
		// 取前5个字符和后5个字符，中间用省略号连接
		// 确保不会越界
		if len(name) > 10 {
			displayName = name[:5] + "..." + name[len(name)-5:]
		}
	}

	progress := float64(downloaded) / float64(total) * 100
	barLength := 50 // 进度条长度
	filledLength := int(progress / 100 * float64(barLength))
	emptyLength := barLength - filledLength
	if emptyLength < 0 {
		emptyLength = 0
	}
	// 构建进度条字符串
	bar := strings.Repeat("=", filledLength) + strings.Repeat(" ", emptyLength)
	downloadedMB := float64(downloaded) / 1024 / 1024
	totalMB := float64(total) / 1024 / 1024

	// 输出进度（覆盖当前行）
	fmt.Printf("\r%s [%-50s] %.1f%% (%.2f/%.2f MB)", displayName, bar, progress, downloadedMB, totalMB)
}

func getDefaultFilename(fileUrl string) string {
	// 从 URL 路径中提取文件名
	segments := strings.Split(fileUrl, "/")
	filename := segments[len(segments)-1]

	// 处理 URL 参数（去掉 ? 后面的内容）
	if idx := strings.Index(filename, "?"); idx != -1 {
		filename = filename[:idx]
	}

	// 确保文件扩展名为 .pdf
	if !strings.HasSuffix(strings.ToLower(filename), ".pdf") {
		filename += ".pdf"
	}

	// 避免文件名为空
	if filename == "" || filename == ".pdf" {
		filename = fmt.Sprintf("download_%d.pdf", time.Now().Unix())
	}
	// 新名字
	newName, err := url.PathUnescape(filename)
	if err == nil {
		filename = newName
	}
	//去掉名字前后的数字
	if strings.Contains(filename, "_") {
		fex := filepath.Ext(filename)
		fname := strings.TrimSuffix(filename, fex)
		nameItems := strings.Split(fname, "_")
		var newNameItems []string
		for _, item := range nameItems {
			if item == "" {
				continue
			}
			_, errs := strconv.ParseInt(item, 10, 64)
			if errs == nil {
				continue
			}
			newNameItems = append(newNameItems, item)
		}
		filename = strings.Join(newNameItems, "_") + fex
	}

	return filename
}
