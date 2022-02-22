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
//go:build !enterprise
// +build !enterprise

package replication

import (
	api "github.com/arangodb/kube-arangodb/pkg/apis/replication/v1"
	"github.com/arangodb/kube-arangodb/pkg/util/errors"
)

// DeploymentReplication is the in process state of an ArangoDeploymentReplication.
type DeploymentReplication struct {
	apiObject *api.ArangoDeploymentReplication // API object
	status    api.DeploymentReplicationStatus  // Internal status of the CR
	config    Config
	deps      Dependencies
}

// New creates a new DeploymentReplication from the given API object.
func New(config Config, deps Dependencies, apiObject *api.ArangoDeploymentReplication) (*DeploymentReplication, error) {
	if err := apiObject.Spec.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}
	dr := &DeploymentReplication{
		apiObject: apiObject,
		status:    *(apiObject.Status.DeepCopy()),
		config:    config,
		deps:      deps,
	}
	return dr, nil
}

// Update the deployment replication.
// This sends an update event in the event queue.
func (dr *DeploymentReplication) Update(apiObject *api.ArangoDeploymentReplication) {
	log := dr.deps.Log
	log.Warn().Msg("ArangoDeploymentReplication Update operation is not available in community version")
}

// Delete the deployment replication.
// Called when the local storage was deleted by the user.
func (dr *DeploymentReplication) Delete() {
	log := dr.deps.Log
	log.Warn().Msg("ArangoDeploymentReplication Delete operation is not available in community version")
}
