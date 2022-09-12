package shadowAPI

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/config"
	"golang.org/x/exp/slices"
)

type Checker interface {
	Check(ctx *fasthttp.RequestCtx) error
}

type ShadowAPI struct {
	Config *config.ShadowAPI
	Logger *logrus.Logger
}

func New(config *config.ShadowAPI, logger *logrus.Logger) Checker {
	return &ShadowAPI{
		Config: config,
		Logger: logger,
	}
}

func (s *ShadowAPI) Check(ctx *fasthttp.RequestCtx) error {
	statusCode := ctx.Response.StatusCode()
	idx := slices.IndexFunc(s.Config.ExcludeList, func(c int) bool { return c == statusCode })
	if idx < 0 {
		s.Logger.WithFields(logrus.Fields{
			"request_id":      fmt.Sprintf("#%016X", ctx.ID()),
			"status_code":     ctx.Response.StatusCode(),
			"response_length": fmt.Sprintf("%d", ctx.Response.Header.ContentLength()),
			"method":          fmt.Sprintf("%s", ctx.Request.Header.Method()),
			"path":            fmt.Sprintf("%s", ctx.Path()),
			"client_address":  ctx.RemoteAddr(),
		}).Error("Shadow API detected")
	}
	return nil
}
