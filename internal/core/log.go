package core

import (
	"fmt"
	"log"
)

const (
	CRITICAL = 50
	ERROR    = 40
	WARNING  = 30
	INFO     = 20
	DEBUG    = 10
)

type Logger struct {
	logger    *log.Logger
	verbosity int
}

func (cl *Logger) Log(verb int, format string, v ...interface{}) error {
	if verb >= cl.verbosity {
		return cl.logger.Output(2, fmt.Sprintf(format, v...))
	}
	return nil
}

func (cl *Logger) log(verb int, format string, v ...interface{}) error {
	if verb >= cl.verbosity {
		return cl.logger.Output(3, fmt.Sprintf(format, v...))
	}
	return nil
}

func (cl *Logger) Critical(s string, v ...interface{}) error {
	return cl.log(CRITICAL, "[CRITICAL] "+s, v...)
}

func (cl *Logger) Error(s string, v ...interface{}) error {
	return cl.log(ERROR, "[ERROR] "+s, v...)
}

func (cl *Logger) Warning(s string, v ...interface{}) error {
	return cl.log(WARNING, "[WARNING] "+s, v...)
}

func (cl *Logger) Info(s string, v ...interface{}) error {
	return cl.log(INFO, "[INFO] "+s, v...)
}

func (cl *Logger) Debug(s string, v ...interface{}) error {
	return cl.log(DEBUG, "[DEBUG] "+s, v...)
}

func NewCondLogger(logger *log.Logger, verbosity int) *Logger {
	return &Logger{verbosity: verbosity, logger: logger}
}
