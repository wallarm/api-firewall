package main

import (
	"os"
	"strings"

	"github.com/ardanlabs/conf"
	"github.com/sirupsen/logrus"

	handlersAPI "github.com/wallarm/api-firewall/cmd/api-firewall/internal/handlers/api"
	handlersGQL "github.com/wallarm/api-firewall/cmd/api-firewall/internal/handlers/graphql"
	handlersProxy "github.com/wallarm/api-firewall/cmd/api-firewall/internal/handlers/proxy"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/web"
	"github.com/wallarm/api-firewall/internal/version"
)

const (
	logPrefix = "main"
)

func main() {
	logger := logrus.New()

	logger.SetLevel(logrus.DebugLevel)

	cFormatter := &config.CustomFormatter{}
	cFormatter.DisableQuote = true
	cFormatter.FullTimestamp = true
	cFormatter.DisableLevelTruncation = true

	logger.SetFormatter(cFormatter)

	// if MODE var has invalid value then proxy mode will be used
	var currentMode config.APIFWMode
	if err := conf.Parse(os.Args[1:], version.Namespace, &currentMode); err != nil {
		if err := handlersProxy.Run(logger); err != nil {
			logger.Infof("%s: error: %s", logPrefix, err)
			os.Exit(1)
		}
		return
	}

	// if MODE var has valid or default value then an appropriate mode will be used
	switch strings.ToLower(currentMode.Mode) {
	case web.APIMode:
		if err := handlersAPI.Run(logger); err != nil {
			logger.Infof("%s: error: %s", logPrefix, err)
			os.Exit(1)
		}
	case web.GraphQLMode:
		if err := handlersGQL.Run(logger); err != nil {
			logger.Infof("%s: error: %s", logPrefix, err)
			os.Exit(1)
		}
	default:
		if err := handlersProxy.Run(logger); err != nil {
			logger.Infof("%s: error: %s", logPrefix, err)
			os.Exit(1)
		}
	}

}
