// Copyright 2023 Juan Pablo Tosso and the OWASP Coraza contributors
// SPDX-License-Identifier: Apache-2.0

package plugins_test

import (
	"testing"

	"github.com/wallarm/api-firewall/internal/modsec/actions"
	"github.com/wallarm/api-firewall/internal/modsec/experimental/plugins"
	"github.com/wallarm/api-firewall/internal/modsec/experimental/plugins/plugintypes"
)

func TestAction(t *testing.T) {
	t.Run("get existing action", func(t *testing.T) {
		action := func() plugintypes.Action {
			return nil
		}

		plugins.RegisterAction("custom_action", action)
		_, err := actions.Get("custom_action")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}