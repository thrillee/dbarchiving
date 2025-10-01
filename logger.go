package main

import (
	"fmt"
	"log"
	"os"
	"time"
)

type Logger struct {
	*log.Logger
}

func NewLogger() *Logger {
	logFile, err := os.OpenFile(
		fmt.Sprintf("archive_%s.log", time.Now().Format("20060102_150405")),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0o666,
	)
	if err != nil {
		log.Fatal("Failed to open log file:", err)
	}

	return &Logger{
		Logger: log.New(logFile, "", log.LstdFlags),
	}
}

func (l *Logger) Info(format string, v ...any) {
	msg := fmt.Sprintf("[INFO] "+format, v...)
	l.Println(msg)
	fmt.Println(msg)
}

func (l *Logger) Error(format string, v ...any) {
	msg := fmt.Sprintf("[ERROR] "+format, v...)
	l.Println(msg)
	fmt.Println(msg)
}

func (l *Logger) Warning(format string, v ...any) {
	msg := fmt.Sprintf("[WARNING] "+format, v...)
	l.Println(msg)
	fmt.Println(msg)
}
