// Copyright 2023 Juan Pablo Tosso and the OWASP Coraza contributors
// SPDX-License-Identifier: Apache-2.0

package collections

import (
	"github.com/wallarm/api-firewall/internal/modsec/collection"
	"github.com/wallarm/api-firewall/internal/modsec/types"
)

var Noop collection.Collection = &noop{}

type noop struct{}

func (c *noop) FindAll() []types.MatchData {
	return nil
}

func (c *noop) Name() string {
	return ""
}
