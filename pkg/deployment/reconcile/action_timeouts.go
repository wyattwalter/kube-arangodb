//
// DISCLAIMER
//
// Copyright 2016-2022 ArangoDB GmbH, Cologne, Germany
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Copyright holder is ArangoDB GmbH, Cologne, Germany
//

package reconcile

import (
	"time"

	api "github.com/arangodb/kube-arangodb/pkg/apis/deployment/v1"
)

func GetActionTimeout(spec api.DeploymentSpec, t api.ActionType) time.Duration {
	if d, ok := getActionTimeout(spec, t); ok {
		return d
	}

	if d, ok := getActionTimeout(spec, "default"); ok {
		return d
	}

	return defaultTimeout
}

func getActionTimeout(spec api.DeploymentSpec, t api.ActionType) (time.Duration, bool) {
	if timeouts := spec.Timeouts; timeouts != nil {
		if d, ok := timeouts.Actions[t]; ok {
			return d.Duration, true
		}
	}

	if d, ok := actionTimeouts[t]; ok {
		return d.Duration, true
	}

	return 0, false
}
