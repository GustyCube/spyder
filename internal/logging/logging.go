package logging

import "go.uber.org/zap"

type Logger = zap.SugaredLogger

func New() *Logger {
	l, _ := zap.NewProduction()
	return l.Sugar()
}
