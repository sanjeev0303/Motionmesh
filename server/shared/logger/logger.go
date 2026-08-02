package logger

import (
	"log"
	"os"
)

type Logger struct {
	infoLog  *log.Logger
	errorLog *log.Logger
}

func New() *Logger {
	return &Logger{
		infoLog:  log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime|log.LUTC|log.Lshortfile),
		errorLog: log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.LUTC|log.Lshortfile),
	}
}

func (l *Logger) Info(format string, v ...interface{}) {
	l.infoLog.Printf(format, v...)
}

func (l *Logger) Error(format string, v ...interface{}) {
	l.errorLog.Printf(format, v...)
}
