package main

import (
	"encoding/json"
	"flag"
	"fmt"
	log "github.com/atpxu/gokits/logger"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gorilla/websocket"
)

type Config struct {
	ListenPort int               `json:"listen_port"`
	Services   map[string]string `json:"services"`
}

var logger = log.GetLogger("wsproxy")

func loadConfig(filePath string) (*Config, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("无法打开配置文件: %v", err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
		}
	}(file)

	var config Config
	decoder := json.NewDecoder(file)
	if err2 := decoder.Decode(&config); err2 != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}
	for path, targetURL := range config.Services {
		logger.Debugf("ws代理: %s -> %s", path, targetURL)
	}
	return &config, nil
}

func proxyWebSocket(w http.ResponseWriter, r *http.Request, targetURL string) {
	// 连接到目标 WebSocket 服务
	logger.Debug("连接到目标 WebSocket 服务:", targetURL)
	u, _ := url.Parse(targetURL)
	targetConn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		logger.Error("连接目标 WebSocket 失败:", err)
		http.Error(w, "无法连接目标服务", http.StatusInternalServerError)
		return
	}
	defer func(targetConn *websocket.Conn) {
		err2 := targetConn.Close()
		if err2 != nil {
		}
	}(targetConn)

	// 将 HTTP 请求升级为 WebSocket
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	clientConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("升级客户端连接失败:", err)
		return
	}
	defer func(clientConn *websocket.Conn) {
		err2 := clientConn.Close()
		if err2 != nil {
		}
	}(clientConn)

	// 开始转发数据
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			messageType, message, err2 := targetConn.ReadMessage()
			if err2 != nil {
				logger.Error("从目标读取数据出错:", err)
				return
			}
			err2 = clientConn.WriteMessage(messageType, message)
			if err2 != nil {
				logger.Error("向目标端写入数据出错:", err)
				return
			}
		}
	}()
	for {
		messageType, message, err2 := clientConn.ReadMessage()
		if err2 != nil {
			logger.Error("从客户端读取数据出错:", err)
			return
		}
		err2 = targetConn.WriteMessage(messageType, message)
		if err2 != nil {
			logger.Error("向客户端写入数据出错:", err)
			return
		}
	}
}

func main() {
	// 通过命令行解析配置文件路径
	configFile := flag.String("config", "config.json", "配置文件路径")
	flag.Parse()

	// 从配置文件加载配置
	logger.Infof("加载配置文件: %s", *configFile)
	config, err := loadConfig(*configFile)
	if err != nil {
		logger.Fatalf("加载配置失败: %v", err)
	}

	// 配置 WebSocket 服务的路由
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSuffix(r.URL.Path, "/")
		targetURL, exists := config.Services[path]
		if !exists {
			http.Error(w, "服务未找到", http.StatusNotFound)
			return
		}

		logger.Debugf("代理请求: %s -> %s\n", path, targetURL)
		proxyWebSocket(w, r, targetURL)
	})

	// 启动服务器
	// Notice, accept ws:// only, wss:// is not supported yet
	address := fmt.Sprintf(":%d", config.ListenPort)
	logger.Infof("WebSocket 代理启动，监听端口 %s", address)
	logger.Fatal(http.ListenAndServe(address, nil))
}
