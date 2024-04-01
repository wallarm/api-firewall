package config

import (
	"errors"
	"os"
	"path"

	"github.com/sirupsen/logrus"
	coraza "github.com/wallarm/api-firewall/internal/modsec"
	"github.com/wallarm/api-firewall/internal/modsec/types"
)

func LoadModSecurityConfiguration(logger *logrus.Logger, cfg *ModSecurity) (coraza.WAF, error) {

	logErr := func(error types.MatchedRule) {
		logger.WithFields(logrus.Fields{
			"tags":     error.Rule().Tags(),
			"version":  error.Rule().Version(),
			"severity": error.Rule().Severity(),
			"rule_id":  error.Rule().ID(),
			"file":     error.Rule().File(),
			"line":     error.Rule().Line(),
			"maturity": error.Rule().Maturity(),
			"accuracy": error.Rule().Accuracy(),
			"uri":      error.URI(),
		}).Error(error.Message())
	}

	var waf coraza.WAF
	var err error

	if cfg.ConfFile != "" || cfg.RulesDir != "" {

		wafConfig := coraza.NewWAFConfig().WithErrorCallback(logErr)

		if cfg.ConfFile != "" {
			if _, err := os.Stat(cfg.ConfFile); os.IsNotExist(err) {
				return nil, errors.New("Loading ModSecurity configuration file error: no such file or directory: " + cfg.ConfFile)
			}
			wafConfig.WithDirectivesFromFile(cfg.ConfFile)
		}

		if cfg.RulesDir != "" {
			if _, err := os.Stat(cfg.RulesDir); os.IsNotExist(err) {
				return nil, errors.New("Loading ModSecurity rules from dir error: no such file or directory: " + cfg.RulesDir)
			}

			rules := path.Join(cfg.RulesDir, "*.conf")
			wafConfig.WithDirectivesFromFile(rules)
		}

		waf, err = coraza.NewWAF(wafConfig)
		if err != nil {
			return nil, err
		}
	}

	return waf, nil
}
