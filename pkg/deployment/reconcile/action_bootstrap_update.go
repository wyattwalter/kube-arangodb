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
	"context"

	api "github.com/arangodb/kube-arangodb/pkg/apis/deployment/v1"
)

func init() {
	registerAction(api.ActionTypeBootstrapUpdate, newBootstrapUpdateAction, defaultTimeout)
}

func newBootstrapUpdateAction(action api.Action, actionCtx ActionContext) Action {
	a := &actionBootstrapUpdate{}

	a.actionImpl = newActionImplDefRef(action, actionCtx)

	return a
}

// actionBackupRestoreClean implements an BackupRestoreClean.
type actionBootstrapUpdate struct {
	// actionImpl implement timeout and member id functions
	actionImpl

	actionEmptyCheckProgress
}

func (a actionBootstrapUpdate) Start(ctx context.Context) (bool, error) {
	if err := a.actionCtx.WithStatusUpdate(ctx, func(status *api.DeploymentStatus) bool {
		if errMessage, ok := a.action.GetParam("error"); ok {
			status.Conditions.Update(api.ConditionTypeBootstrapCompleted, true, "Bootstrap failed", errMessage)
			status.Conditions.Update(api.ConditionTypeBootstrapSucceded, false, "Bootstrap failed", errMessage)
		} else {
			status.Conditions.Update(api.ConditionTypeBootstrapCompleted, true, "Bootstrap successful", "The bootstrap process has been completed successfully")
			status.Conditions.Update(api.ConditionTypeBootstrapSucceded, true, "Bootstrap successful", "The bootstrap process has been completed successfully")
		}
		return true
	}); err != nil {
		return false, err
	}

	return true, nil
}
