package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

// 存储不同名称的 Logger 实例
var (
	logInstances = make(map[string]*logrus.Logger)
	fileHandles  = make(map[string]*os.File)
	onceCloseAll sync.Once
	mutex        sync.Mutex
)

type loggerConfig struct {
	name         string
	level        logrus.Level
	dest         string // "stdout", "stderr", or file path
	alsoToStdout bool   // when output dest is file path, also output to stdout
}

// initLogger 根据配置初始化 Logger（私有方法）
func initLogger(config loggerConfig) *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(config.level)

	var output io.Writer

	// 设置输出到文件或 stdout
	if config.dest == "stdout" || config.dest == "stderr" {
		output = os.Stdout
		if config.dest == "stderr" {
			output = os.Stderr
		}
		logger.SetFormatter(&colorFormatter{})
	} else {
		// 如果指定文件路径，则打开文件
		file, err := os.OpenFile(config.dest, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			fmt.Printf("Failed to open log file: %v", err)
			output = os.Stdout // 如果文件无法打开，回退到 stdout
			logger.SetFormatter(&colorFormatter{})
		} else {
			// 创建 MultiWriter：同时输出到 stdout 和文件
			// no color format for file and stdout
			if config.alsoToStdout {
				output = io.MultiWriter(os.Stdout, file)
			} else {
				output = file
			}
			logger.SetFormatter(&plainFormatter{})
			fileHandles[config.name] = file
		}
	}

	logger.SetOutput(output)
	logInstances[config.name] = logger
	return logger
}

// getConfigedLogger 根据配置获取或创建 Logger 实例
func getConfigedLogger(config loggerConfig) *logrus.Logger {
	// 第一次无锁检查
	if logger, exists := logInstances[config.name]; exists {
		return logger
	}

	// 加锁并双重检查
	mutex.Lock()
	defer mutex.Unlock()

	if logger, exists := logInstances[config.name]; exists {
		return logger
	}
	initCloseHook()
	return initLogger(config)
}

func GetLogger(name string) *logrus.Logger {
	return getConfigedLogger(loggerConfig{
		name:  name,
		level: logrus.DebugLevel,
		dest:  "stdout",
	})
}

func GetFileLogger(name string, filePath string, alsoToStdout bool) *logrus.Logger {
	return getConfigedLogger(loggerConfig{
		name:         name,
		level:        logrus.DebugLevel,
		dest:         filePath,
		alsoToStdout: alsoToStdout,
	})
}

type plainFormatter struct{} // no color for plainFormatter
type colorFormatter struct{} // colorFormatter 自定义格式化器，用于为不同日志级别设置颜色

func (f *plainFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	level := strings.ToUpper(entry.Level.String())
	message := fmt.Sprintf("[%s][%s]%s\n", timestamp, level, entry.Message)
	return []byte(message), nil
}

func (f *colorFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	level := colorizeLevel(entry.Level.String())
	message := fmt.Sprintf("[%s][%s]%s\n", timestamp, level, entry.Message)
	return []byte(message), nil
}

// colorizeLevel 仅为日志级别上色
func colorizeLevel(level string) string {
	switch level {
	case "info":
		return "\033[32mINFO\033[0m" // 绿色
	case "warning":
		return "\033[33mWARN\033[0m" // 黄色
	case "error":
		return "\033[31mERROR\033[0m" // 红色
	default:
		return strings.ToUpper(level)
	}
}

// CloseAllLoggers 关闭所有 Logger 的文件句柄
func CloseAllLoggers() {
	onceCloseAll.Do(func() {
		mutex.Lock()
		defer mutex.Unlock()

		for name, file := range fileHandles {
			if err := file.Close(); err != nil {
				fmt.Printf("Failed to close log file for logger '%s': %v\n", name, err)
			}
			log.Print("Closed log file: ", name)
			delete(fileHandles, name)
		}
		logInstances = make(map[string]*logrus.Logger) // 清空实例
	})
}

// initCloseHook 注册一个系统信号监听器，在程序退出时调用 CloseAllLoggers
func initCloseHook() {
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan // 等待信号

		CloseAllLoggers() // 捕获信号后关闭所有日志文件
		os.Exit(0)        // 正常退出程序
	}()
}
