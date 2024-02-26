// Copyright 2023 Juan Pablo Tosso and the OWASP Coraza contributors
// SPDX-License-Identifier: Apache-2.0

package plugins

import (
	"github.com/wallarm/api-firewall/internal/modsec/actions"
	"github.com/wallarm/api-firewall/internal/modsec/experimental/plugins/plugintypes"
)

// ActionFactory is used to wrap a RuleAction so that it can be registered
// and recreated on each call
type ActionFactory = func() plugintypes.Action

// RegisterAction registers a new RuleAction
// If you register an action with an existing name, it will be overwritten.
func RegisterAction(name string, a ActionFactory) {
	actions.Register(name, a)
}
