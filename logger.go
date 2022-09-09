package forwardBot

import (
	"github.com/sirupsen/logrus"
	"os"
)

var (
	//日志
	logger *logrus.Logger
)

func init() {
	logger = logrus.New()
	logger.Out = os.Stdout
	logger.SetReportCaller(true)
}

// SetLogger 设置日志器
func SetLogger(l *logrus.Logger) {
	logger = l
}
