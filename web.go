package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

//go:embed templates/*
var templatesFS embed.FS

// WebServer Web服务结构体
type WebServer struct {
	config     *Config
	configPath string
	template   *template.Template
	server     *http.Server
	progress   map[string]*DownloadProgress
	mu         sync.RWMutex
	lastActive time.Time
	exitChan   chan bool
	clients    map[*websocket.Conn]bool // WebSocket客户端连接
	clientsMu  sync.RWMutex             // 保护clients的互斥锁
}

// DownloadProgress 下载进度信息
type DownloadProgress struct {
	TaskID     string  `json:"task_id"`
	Filename   string  `json:"filename"`
	Percent    float64 `json:"percent"`
	Downloaded int64   `json:"downloaded"`
	Total      int64   `json:"total"`
	Status     string  `json:"status"` // pending, downloading, completed, failed
	OutputPath string  `json:"output_path"`
	ErrorMsg   string  `json:"error_msg,omitempty"` // 错误信息
}

// WebSocket升级器
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源
	},
}

// NewWebServer 创建新的Web服务器实例
func NewWebServer(config *Config, configPath string) *WebServer {
	// 解析嵌入的模板文件
	tmpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		panic(fmt.Sprintf("无法解析模板文件: %v", err))
	}

	server := &WebServer{
		config:     config,
		configPath: configPath,
		template:   tmpl,
		progress:   make(map[string]*DownloadProgress),
		lastActive: time.Now(),
		exitChan:   make(chan bool, 1),
		clients:    make(map[*websocket.Conn]bool),
	}

	// 启动自动退出检查协程
	go server.autoExitChecker()

	return server
}

// autoExitChecker 自动退出检查器
func (ws *WebServer) autoExitChecker() {
	ticker := time.NewTicker(30 * time.Second) // 每30秒检查一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 检查是否超过5分钟无活动
			if time.Since(ws.lastActive) > 5*time.Minute {
				fmt.Println("程序因长时间无操作自动退出")
				os.Exit(0)
			}
		case <-ws.exitChan:
			// 收到退出信号，直接退出
			return
		}
	}
}

// updateLastActive 更新最后活动时间
func (ws *WebServer) updateLastActive() {
	ws.lastActive = time.Now()
}

// Start 启动Web服务
func (ws *WebServer) Start(port string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ws.updateLastActive()
		ws.handleIndex(w, r)
	})
	mux.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
		ws.updateLastActive()
		ws.handleDownload(w, r)
	})
	mux.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		ws.updateLastActive()
		ws.handleGetConfig(w, r)
	})
	mux.HandleFunc("/save-config", func(w http.ResponseWriter, r *http.Request) {
		ws.updateLastActive()
		ws.handleSaveConfig(w, r)
	})
	mux.HandleFunc("/reset-config", func(w http.ResponseWriter, r *http.Request) {
		ws.updateLastActive()
		ws.handleResetConfig(w, r)
	})
	mux.HandleFunc("/exit", func(w http.ResponseWriter, r *http.Request) {
		ws.updateLastActive()
		ws.handleExit(w, r)
	})
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws.updateLastActive()
		ws.handleWebSocket(w, r)
	})
	mux.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		ws.updateLastActive()
		ws.handleStatic(w, r)
	})

	ws.server = &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	fmt.Printf("Web服务器启动成功，访问地址: http://localhost:%s\n", port)
	return ws.server.ListenAndServe()
}

// handleWebSocket 处理WebSocket连接
func (ws *WebServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("WebSocket升级失败: %v\n", err)
		return
	}
	defer conn.Close()

	// 添加客户端到连接池
	ws.clientsMu.Lock()
	ws.clients[conn] = true
	ws.clientsMu.Unlock()
	defer func() {
		ws.clientsMu.Lock()
		delete(ws.clients, conn)
		ws.clientsMu.Unlock()
	}()

	// 监听客户端消息（这里主要是为了保持连接）
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// broadcastProgress 广播进度更新到所有WebSocket客户端
func (ws *WebServer) broadcastProgress(progress *DownloadProgress) {
	ws.clientsMu.RLock()
	defer ws.clientsMu.RUnlock()

	// 序列化进度数据
	data, err := json.Marshal(progress)
	if err != nil {
		fmt.Printf("序列化进度数据失败: %v\n", err)
		return
	}

	// 发送给所有连接的客户端
	for client := range ws.clients {
		err := client.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			// 如果发送失败，标记连接为断开
			fmt.Printf("向客户端发送消息失败: %v\n", err)
			client.Close()
		}
	}
}

// handleIndex 处理首页请求
func (ws *WebServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	err := ws.template.ExecuteTemplate(w, "index.html", ws.config)
	if err != nil {
		fmt.Printf("模板执行错误: %v\n", err)
		// 检查是否已经写入了响应头
		if w.Header().Get("Content-Type") == "" {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}
}

// handleSaveConfig 处理保存配置请求
func (ws *WebServer) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析JSON数据
	var data Config
	if err := parseJSON(r, &data); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	ws.config = &data

	// 保存配置到文件
	if err := SaveConfig(ws.configPath, ws.config); err != nil {
		sendJSONResponse(w, map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("保存配置失败: %v", err),
		})
		return
	}

	sendJSONResponse(w, map[string]interface{}{
		"success": true,
		"message": "配置保存成功",
	})
}

// handleResetConfig 处理恢复默认配置请求
func (ws *WebServer) handleResetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 创建默认配置
	defaultConfig := getDefaultConfig()
	// 保存默认配置到文件
	if err := SaveConfig(ws.configPath, defaultConfig); err != nil {
		sendJSONResponse(w, map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("保存默认配置失败: %v", err),
		})
		return
	}

	// 更新当前配置
	ws.config = defaultConfig

	sendJSONResponse(w, map[string]interface{}{
		"success": true,
		"message": "已恢复默认配置",
	})
}

// handleDownload 处理下载请求
func (ws *WebServer) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析JSON数据，获取URL
	var requestData map[string]interface{}
	if err := parseJSON(r, &requestData); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 获取URL
	url, ok := requestData["url"].(string)
	if !ok || url == "" {
		// 如果请求中没有提供URL，则使用配置中的URL
		url = ws.config.URL
	}

	// 检查URL是否为空
	if url == "" {
		sendJSONResponse(w, map[string]interface{}{
			"success": false,
			"message": "请提供PDF文件URL",
		})
		return
	}

	// 设置默认输出路径
	downloadConfig := ws.config.Copy()
	downloadConfig.URL = url
	var fileName string
	if downloadConfig.OutputPath == "" {
		fileName = getDefaultFilename(url)
		downloadConfig.OutputPath = filepath.Join(downloadConfig.OutputDir, fileName)
	} else {
		fileName = filepath.Base(url)
	}
	timeout := downloadConfig.GetTimeoutDuration()

	// 添加HTTP请求头
	header := ws.config.Headers
	downloadConfig.Headers = make(map[string]string)
	for k, v := range header {
		downloadConfig.Headers[k] = v
	}

	// 创建上下文
	ctx, _ := context.WithTimeout(context.Background(), timeout)

	// 生成任务ID
	taskID := fmt.Sprintf("%d", time.Now().UnixNano())

	// 从URL中提取文件名
	filename := getDefaultFilename(url)

	// 创建初始进度信息
	progress := &DownloadProgress{
		TaskID:     taskID,
		Filename:   filename,
		Percent:    0,
		Downloaded: 0,
		Total:      0,
		Status:     "pending",
		OutputPath: ws.config.OutputPath,
	}

	// 广播初始进度
	ws.broadcastProgress(progress)

	// 在goroutine中执行下载（带进度回调），这样可以立即返回任务信息
	go func() {
		// 执行下载（带进度回调）
		err := downloadPDFWithProgress(ctx, *downloadConfig, func(percent float64, downloaded, total int64) {
			// 更新进度信息
			progress.Percent = percent
			progress.Downloaded = downloaded
			progress.Total = total
			progress.Status = "downloading"

			// 广播进度更新
			ws.broadcastProgress(progress)
		})

		// 下载完成后更新状态
		if err != nil {
			progress.Status = "failed"
			progress.Percent = 0
			progress.ErrorMsg = err.Error()
			ws.broadcastProgress(progress)
		} else {
			progress.Status = "completed"
			progress.Percent = 100
			ws.broadcastProgress(progress)
		}
	}()

	// 立即返回任务信息
	sendJSONResponse(w, map[string]interface{}{
		"success":     true,
		"task_id":     taskID,
		"filename":    filename,
		"output_path": ws.config.OutputPath,
		"total_size":  0, // 总大小将在下载开始后通过WebSocket更新
		"status":      "pending",
		"message":     "下载任务已启动",
	})
}

// handleExit 处理退出程序请求
func (ws *WebServer) handleExit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sendJSONResponse(w, map[string]interface{}{
		"success": true,
		"message": "程序正在退出",
	})

	// 发送退出信号
	select {
	case ws.exitChan <- true:
	default:
	}

	// 在goroutine中退出程序，以允许响应返回给客户端
	go func() {
		// 给一点时间让响应返回
		time.Sleep(1 * time.Second)
		fmt.Println("程序正在退出...")
		os.Exit(0)
	}()
}

// handleStatic 处理静态文件请求
func (ws *WebServer) handleStatic(w http.ResponseWriter, r *http.Request) {
	// 简单的静态文件处理
	http.NotFound(w, r)
}

// handleGetConfig 处理获取配置请求
func (ws *WebServer) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 构造返回数据
	configData := map[string]interface{}{
		"url":         ws.config.URL,
		"output_path": ws.config.OutputPath,
		"output_dir":  ws.config.OutputDir,
		"timeout":     ws.config.Timeout,
		"chunk_size":  ws.config.ChunkSize,
		"headers":     ws.config.Headers,
	}

	sendJSONResponse(w, configData)
}

// parseJSON 解析JSON请求体
func parseJSON(r *http.Request, v interface{}) error {
	decoder := json.NewDecoder(r.Body)
	return decoder.Decode(v)
}

// sendJSONResponse 发送JSON响应
func sendJSONResponse(w http.ResponseWriter, data interface{}) {
	// 检查是否已经设置了Content-Type头部
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}
	if err := json.NewEncoder(w).Encode(data); err != nil {
		fmt.Printf("JSON编码错误: %v\n", err)
	}
}
