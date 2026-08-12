package log

import (
	"io"
	"os"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/sirupsen/logrus"
)

var Logger = logrus.New()

func InitLogger(logPath string) {
	os.MkdirAll(logPath, 0755)

	Logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "@timestamp",
			logrus.FieldKeyLevel: "@level",
			logrus.FieldKeyMsg:   "@message",
		},
	})

	if logPath != "" {
		writer, err := rotatelogs.New(
			logPath+"/app.%Y%m%d.log",
			rotatelogs.WithMaxAge(7*24*time.Hour),
			rotatelogs.WithRotationTime(24*time.Hour),
		)
		if err == nil {
			Logger.SetOutput(io.MultiWriter(os.Stdout, writer))
			return
		}
	}

	Logger.SetOutput(os.Stdout)
	Logger.SetLevel(logrus.InfoLevel)
}

func Info(args ...interface{})  { Logger.Info(args...) }
func Infof(format string, args ...interface{}) { Logger.Infof(format, args...) }
func Error(args ...interface{}) { Logger.Error(args...) }
func Warn(args ...interface{})  { Logger.Warn(args...) }
func Debug(args ...interface{}) { Logger.Debug(args...) }

func WithFields(fields logrus.Fields) *logrus.Entry { return Logger.WithFields(fields) }
