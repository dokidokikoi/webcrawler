package log

import "go.uber.org/zap"

var logger *zap.Logger

func init() {
	var err error
	logger, err = zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
}

func L() *zap.Logger {
	return logger
}
