// Copyright 2023 Juan Pablo Tosso and the OWASP Coraza contributors
// SPDX-License-Identifier: Apache-2.0

package plugins_test

import (
	"testing"

	"github.com/wallarm/api-firewall/internal/modsec/experimental/plugins"
	"github.com/wallarm/api-firewall/internal/modsec/transformations"
)

func TestTransformation(t *testing.T) {
	t.Run("get existing transformation", func(t *testing.T) {
		transformation := func(input string) (string, bool, error) {
			return "", false, nil
		}

		plugins.RegisterTransformation("custom_transformation", transformation)
		_, err := transformations.GetTransformation("custom_transformation")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}