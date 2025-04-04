package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ardanlabs/conf"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

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

	// read logs related env params
	var cfgInit config.APIFWInit
	cfgInit.Version.SVN = version.Version
	cfgInit.Version.Desc = version.ProjectName

	if err := conf.Parse(os.Args[1:], version.Namespace, &cfgInit); err != nil {
		switch err {
		case conf.ErrHelpWanted:
			var usage string
			var err error

			switch strings.ToLower(cfgInit.Mode) {
			case "api":
				var cfg config.APIMode
				usage, err = conf.Usage(version.Namespace, &cfg)
				if err != nil {
					log.Error().Msgf("%s: generating usage: %s", logPrefix, err)
				}
			case "graphql":
				var cfg config.GraphQLMode
				usage, err = conf.Usage(version.Namespace, &cfg)
				if err != nil {
					log.Error().Msgf("%s: generating usage: %s", logPrefix, err)
				}
			default:
				var cfg config.ProxyMode
				usage, err = conf.Usage(version.Namespace, &cfg)
				if err != nil {
					log.Error().Msgf("%s: generating usage: %s", logPrefix, err)
				}
			}

			fmt.Println(usage)
			os.Exit(0)
		case conf.ErrVersionWanted:
			versionStr, err := conf.VersionString(version.Namespace, &cfgInit)
			if err != nil {
				log.Error().Msgf("%s: generating config version: %s", logPrefix, err)
			}
			fmt.Println(versionStr)
			os.Exit(0)
		}

		log.Error().Msgf("%s: error: %s", logPrefix, err)
		os.Exit(1)
	}

	var output zerolog.ConsoleWriter

	switch strings.ToLower(cfgInit.LogFormat) {
	case "json":
		log.Logger = zerolog.New(os.Stderr).
			With().
			Timestamp().
			Logger()
	case "text":
		output = zerolog.ConsoleWriter{
			Out:                 os.Stderr,
			TimeFormat:          time.RFC3339,
			FormatFieldValue:    config.DisableMultiStringFormat,
			FormatMessage:       config.DisableMultiStringFormat,
			FormatErrFieldValue: config.DisableMultiStringFormat,
		}

		log.Logger = zerolog.New(output).
			With().
			Timestamp().
			Logger()
	}

	switch strings.ToLower(cfgInit.LogLevel) {
	case "trace":
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case "warning":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// if MODE var has valid or default value then the corresponding mode will be used
	// default MODE is PROXY
	switch strings.ToLower(cfgInit.Mode) {
	case web.APIMode:
		if err := handlersAPI.Run(log.Logger); err != nil {
			log.Info().Msgf("%s: error: %s", logPrefix, err)
			os.Exit(1)
		}
	case web.GraphQLMode:
		if err := handlersGQL.Run(log.Logger); err != nil {
			log.Info().Msgf("%s: error: %s", logPrefix, err)
			os.Exit(1)
		}
	default:
		if err := handlersProxy.Run(log.Logger); err != nil {
			log.Info().Msgf("%s: error: %s", logPrefix, err)
			os.Exit(1)
		}
	}

}
