package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// LogLevel type
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

// ANSI Color Codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	colorBold   = "\033[1m"
)

// Config struct to hold filtering logic
type Config struct {
	GlobalLevel    LogLevel
	PackageFilters map[string]LogLevel
	FileFilters    map[string]LogLevel
}

var (
	config Config
	mu     sync.RWMutex
	logger *log.Logger
)

func init() {
	// Default configuration
	config = Config{
		GlobalLevel:    INFO, // Default to INFO, silences DEBUG
		PackageFilters: make(map[string]LogLevel),
		FileFilters:    make(map[string]LogLevel),
	}
	logger = log.New(os.Stdout, "", log.Ldate|log.Ltime)
}

// InitLogger allows overriding the default configuration
func InitLogger(c Config) {
	mu.Lock()
	defer mu.Unlock()
	config = c
}

// SetGlobalLevel dynamically sets global level
func SetGlobalLevel(level LogLevel) {
	mu.Lock()
	defer mu.Unlock()
	config.GlobalLevel = level
}

// SetPackageLevel dynamically sets package level
func SetPackageLevel(pkg string, level LogLevel) {
	mu.Lock()
	defer mu.Unlock()
	config.PackageFilters[pkg] = level
}

// SetFileLevel dynamically sets file level
func SetFileLevel(file string, level LogLevel) {
	mu.Lock()
	defer mu.Unlock()
	config.FileFilters[file] = level
}

func getCallerInfo() (string, string) {
	pc, file, _, ok := runtime.Caller(2)
	if !ok {
		return "unknown", "unknown"
	}
	
	// Extract file name
	fileName := filepath.Base(file)
	
	// Extract package name from PC
	funcName := runtime.FuncForPC(pc).Name()
	lastSlash := strings.LastIndexByte(funcName, '/')
	if lastSlash < 0 {
		lastSlash = 0
	}
	firstDot := strings.IndexByte(funcName[lastSlash:], '.') + lastSlash
	
	pkgName := "unknown"
	if firstDot > lastSlash {
		pkgName = funcName[:firstDot]
	}
	
	// Clean up module path from package name if present
	parts := strings.Split(pkgName, "/")
	if len(parts) > 0 {
		pkgName = parts[len(parts)-1]
	}
	
	return pkgName, fileName
}

func shouldLog(level LogLevel, pkg, file string) bool {
	mu.RLock()
	defer mu.RUnlock()

	// 1. File filter has highest priority
	if fileLevel, ok := config.FileFilters[file]; ok {
		return level >= fileLevel
	}

	// 2. Package filter has next priority
	if pkgLevel, ok := config.PackageFilters[pkg]; ok {
		return level >= pkgLevel
	}

	// 3. Fallback to global level
	return level >= config.GlobalLevel
}

func logMessage(level LogLevel, color string, prefix string, v ...interface{}) {
	pkg, file := getCallerInfo()
	
	if !shouldLog(level, pkg, file) {
		return
	}

	msg := fmt.Sprint(v...)
	formattedMsg := fmt.Sprintf("%s[%s][%s] %s%s", color, pkg, file, msg, colorReset)
	
	logger.Println(formattedMsg)
}

func logMessagef(level LogLevel, color string, prefix string, format string, v ...interface{}) {
	pkg, file := getCallerInfo()
	
	if !shouldLog(level, pkg, file) {
		return
	}

	msg := fmt.Sprintf(format, v...)
	formattedMsg := fmt.Sprintf("%s[%s][%s] %s%s", color, pkg, file, msg, colorReset)
	
	logger.Println(formattedMsg)
}

// Debug logs a debug message (Cyan)
func Debug(v ...interface{}) {
	logMessage(DEBUG, colorCyan, "DEBUG", v...)
}

func Debugf(format string, v ...interface{}) {
	logMessagef(DEBUG, colorCyan, "DEBUG", format, v...)
}

// Info logs an info message (Green)
func Info(v ...interface{}) {
	logMessage(INFO, colorGreen, "INFO", v...)
}

func Infof(format string, v ...interface{}) {
	logMessagef(INFO, colorGreen, "INFO", format, v...)
}

// Warn logs a warning message (Yellow)
func Warn(v ...interface{}) {
	logMessage(WARN, colorYellow, "WARN", v...)
}

func Warnf(format string, v ...interface{}) {
	logMessagef(WARN, colorYellow, "WARN", format, v...)
}

// Error logs an error message (Red)
func Error(v ...interface{}) {
	logMessage(ERROR, colorRed, "ERROR", v...)
}

func Errorf(format string, v ...interface{}) {
	logMessagef(ERROR, colorRed, "ERROR", format, v...)
}

// Fatal logs a fatal message and calls os.Exit(1) (Bold Red)
func Fatal(v ...interface{}) {
	logMessage(FATAL, colorBold+colorRed, "FATAL", v...)
	os.Exit(1)
}

func Fatalf(format string, v ...interface{}) {
	logMessagef(FATAL, colorBold+colorRed, "FATAL", format, v...)
	os.Exit(1)
}
