package shadowAPI

import (
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/router"
	"golang.org/x/exp/slices"
)

type Checker interface {
	Check(ctx *fasthttp.RequestCtx) error
}

type ShadowAPI struct {
	Config     *config.ShadowAPI
	Logger     *logrus.Logger
	SwagRouter *router.Router
}

func New(config *config.ShadowAPI, swagRouter *router.Router, logger *logrus.Logger) Checker {
	return &ShadowAPI{
		Config:     config,
		Logger:     logger,
		SwagRouter: swagRouter,
	}
}

func (s *ShadowAPI) Check(ctx *fasthttp.RequestCtx) error {

	currentMethod := fmt.Sprintf("%s", ctx.Request.Header.Method())
	currentPath := fmt.Sprintf("%s", ctx.Path())

	if err := s.CheckMethodAndPath(currentMethod, currentPath); err != nil {
		s.Logger.WithFields(logrus.Fields{
			"request_id":      fmt.Sprintf("#%016X", ctx.ID()),
			"status_code":     ctx.Response.StatusCode(),
			"response_length": fmt.Sprintf("%d", ctx.Response.Header.ContentLength()),
			"method":          currentMethod,
			"path":            currentPath,
			"client_address":  ctx.RemoteAddr(),
		}).Error("Shadow API detected: method not found in the OpenAPI specification")
	}

	statusCode := ctx.Response.StatusCode()
	idx := slices.IndexFunc(s.Config.ExcludeList, func(c int) bool { return c == statusCode })
	if idx < 0 {
		s.Logger.WithFields(logrus.Fields{
			"request_id":      fmt.Sprintf("#%016X", ctx.ID()),
			"status_code":     ctx.Response.StatusCode(),
			"response_length": fmt.Sprintf("%d", ctx.Response.Header.ContentLength()),
			"method":          currentMethod,
			"path":            currentPath,
			"client_address":  ctx.RemoteAddr(),
			"violation":       "shadow_api",
		}).Error("Shadow API detected: response status code not found in the OpenAPI specification")
	}
	return nil
}

func (s *ShadowAPI) CheckMethodAndPath(method, path string) error {

	for _, route := range s.SwagRouter.Routes {
		if method == route.Method && route.Path == path {
			return nil
		}
	}

	for _, route := range s.SwagRouter.Routes {
		if route.Path == path {
			return errors.New("method not found in the OpenAPI specification")
		}
	}

	return errors.New("path not found in the OpenAPI specification")
}

func (s *ShadowAPI) CheckRequestParams(ctx *fasthttp.RequestCtx, method, path string) error {
	// TODO: implement params checker. If parameters in requests are missing then
	for _, route := range s.SwagRouter.Routes {
		if method == route.Method && route.Path == path {
			return nil
		}
	}

	return errors.New("method and path not found in the OpenAPI specification")
}

func (s *ShadowAPI) CheckResponseParams(ctx *fasthttp.RequestCtx, method, path string) error {
	// TODO: implement params checker
	for _, route := range s.SwagRouter.Routes {
		if method == route.Method && route.Path == path {
			return nil
		}
	}

	return errors.New("method and path not found in the OpenAPI specification")
}
