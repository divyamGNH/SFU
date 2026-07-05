package logger

import (
	"log"
	"os"
)

type Level int

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

var currentLevel = DebugLevel

func SetLevel(level Level) {
	currentLevel = level
}

func Debug(format string, args ...any) {
	if currentLevel <= DebugLevel {
		log.Printf("[DEBUG] "+format, args...)
	}
}

func Info(format string, args ...any) {
	if currentLevel <= InfoLevel {
		log.Printf("[INFO] "+format, args...)
	}
}

func Warn(format string, args ...any) {
	if currentLevel <= WarnLevel {
		log.Printf("[WARN] "+format, args...)
	}
}

func Error(format string, args ...any) {
	if currentLevel <= ErrorLevel {
		log.Printf("[ERROR] "+format, args...)
	}
}

func Fatal(format string, args ...any) {
	log.Printf("[FATAL] "+format, args...)
	os.Exit(1)
}
