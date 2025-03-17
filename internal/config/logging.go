package config

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog"
)

func DisableMultiStringFormat(i interface{}) string {
	if value, ok := i.(string); ok {
		return strings.ReplaceAll(value, "\n", " ")
	}
	return ""
}

type ZerologAdapter struct {
	Logger zerolog.Logger
}

// Printf func wrapper
func (z *ZerologAdapter) Printf(format string, args ...interface{}) {
	z.Logger.Info().Msg(fmt.Sprintf(format, args...))
}
