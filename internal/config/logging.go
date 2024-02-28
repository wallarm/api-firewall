package config

import (
	"strings"

	"github.com/sirupsen/logrus"
)

type CustomFormatter struct {
	logrus.TextFormatter
}

func (f *CustomFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	entry.Message = strings.ReplaceAll(entry.Message, "\n", " ")
	l, e := f.TextFormatter.Format(entry)
	return l, e
}
