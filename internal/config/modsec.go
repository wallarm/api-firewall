package config

import (
	"errors"
	"os"
	"path"

	"github.com/corazawaf/coraza/v3"
	"github.com/corazawaf/coraza/v3/types"
	"github.com/rs/zerolog"
)

type ModSecurity struct {
	ConfFiles          []string `conf:"env:MODSEC_CONF_FILES"`
	RulesDir           string   `conf:"env:MODSEC_RULES_DIR"`
	RequestValidation  string   `conf:"env:MODSEC_REQUEST_VALIDATION" validate:"omitempty,oneof=DISABLE BLOCK LOG_ONLY"`
	ResponseValidation string   `conf:"env:MODSEC_RESPONSE_VALIDATION" validate:"omitempty,oneof=DISABLE BLOCK LOG_ONLY"`
}

func LoadModSecurityConfiguration(cfg *ModSecurity, logger zerolog.Logger) (coraza.WAF, error) {

	logErr := func(error types.MatchedRule) {
		logger.Error().
			Strs("tags", error.Rule().Tags()).
			Str("version", error.Rule().Version()).
			Str("severity", error.Rule().Severity().String()).
			Int("rule_id", error.Rule().ID()).
			Str("file", error.Rule().File()).
			Int("line", error.Rule().Line()).
			Int("maturity", error.Rule().Maturity()).
			Int("accuracy", error.Rule().Accuracy()).
			Str("uri", error.URI()).
			Msg(error.Message())
	}

	var waf coraza.WAF
	var err error

	if len(cfg.ConfFiles) > 0 || cfg.RulesDir != "" {

		wafConfig := coraza.NewWAFConfig().WithErrorCallback(logErr)

		if len(cfg.ConfFiles) > 0 {
			for _, confFile := range cfg.ConfFiles {
				if _, err := os.Stat(confFile); os.IsNotExist(err) {
					return nil, errors.New("Loading ModSecurity configuration file error: no such file or directory: " + confFile)
				}
				wafConfig = wafConfig.WithDirectivesFromFile(confFile)
			}
		}

		if cfg.RulesDir != "" {
			if _, err := os.Stat(cfg.RulesDir); os.IsNotExist(err) {
				return nil, errors.New("Loading ModSecurity rules from dir error: no such file or directory: " + cfg.RulesDir)
			}

			rules := path.Join(cfg.RulesDir, "*.conf")
			wafConfig = wafConfig.WithDirectivesFromFile(rules)
		}

		waf, err = coraza.NewWAF(wafConfig)
		if err != nil {
			return nil, err
		}
	}

	return waf, nil
}
