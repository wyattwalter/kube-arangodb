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

package replication

import (
	"context"
	"time"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/arangodb/arangosync-client/client"

	api "github.com/arangodb/kube-arangodb/pkg/apis/replication/v1"
	"github.com/arangodb/kube-arangodb/pkg/generated/clientset/versioned"
	"github.com/arangodb/kube-arangodb/pkg/util/constants"
	"github.com/arangodb/kube-arangodb/pkg/util/errors"
	"github.com/arangodb/kube-arangodb/pkg/util/k8sutil"
)

const (
	maxCancelFailures = 5 // After this amount of failed cancel-synchronization attempts, the operator switch to abort-sychronization.
)

// addFinalizers adds a stop-sync finalizer to the api object when needed.
func (dr *DeploymentReplication) addFinalizers() error {
	apiObject := dr.apiObject
	if apiObject.GetDeletionTimestamp() != nil {
		// Delete already triggered, cannot add.
		return nil
	}
	for _, f := range apiObject.GetFinalizers() {
		if f == constants.FinalizerDeplReplStopSync {
			// Finalizer already added
			return nil
		}
	}
	apiObject.SetFinalizers(append(apiObject.GetFinalizers(), constants.FinalizerDeplReplStopSync))
	if err := dr.updateCRSpec(apiObject.Spec); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

// runFinalizers goes through the list of ArangoDeploymentReplication finalizers to see if they can be removed.
func (dr *DeploymentReplication) runFinalizers(ctx context.Context, p *api.ArangoDeploymentReplication) error {
	log := dr.log.Str("replication-name", p.GetName())
	var removalList []string
	for _, f := range p.ObjectMeta.GetFinalizers() {
		switch f {
		case constants.FinalizerDeplReplStopSync:
			log.Debug("Inspecting stop-sync finalizer")
			if err := dr.inspectFinalizerDeplReplStopSync(ctx, p); err == nil {
				removalList = append(removalList, f)
			} else {
				log.Err(err).Str("finalizer", f).Debug("Cannot remove finalizer yet")
			}
		}
	}
	// Remove finalizers (if needed)
	if len(removalList) > 0 {
		ignoreNotFound := false
		if err := removeDeploymentReplicationFinalizers(dr.deps.Client.Arango(), p, removalList, ignoreNotFound); err != nil {
			log.Err(err).Debug("Failed to update deployment replication (to remove finalizers)")
			return errors.WithStack(err)
		}
	}
	return nil
}

// inspectFinalizerDeplReplStopSync checks the finalizer condition for stop-sync.
// It returns nil if the finalizer can be removed.
func (dr *DeploymentReplication) inspectFinalizerDeplReplStopSync(ctx context.Context, p *api.ArangoDeploymentReplication) error {
	// Inspect phase
	if p.Status.Phase.IsFailed() {
		dr.log.Debug("Deployment replication is already failed, safe to remove stop-sync finalizer")
		return nil
	}

	// Inspect deployment deletion state in source
	abort := dr.status.CancelFailures > maxCancelFailures
	depls := dr.deps.Client.Arango().DatabaseV1().ArangoDeployments(p.GetNamespace())
	if name := p.Spec.Source.GetDeploymentName(); name != "" {
		depl, err := depls.Get(context.Background(), name, meta.GetOptions{})
		if k8sutil.IsNotFound(err) {
			dr.log.Debug("Source deployment is gone. Abort enabled")
			abort = true
		} else if err != nil {
			dr.log.Err(err).Warn("Failed to get source deployment")
			return errors.WithStack(err)
		} else if depl.GetDeletionTimestamp() != nil {
			dr.log.Debug("Source deployment is being deleted. Abort enabled")
			abort = true
		}
	}

	// Inspect deployment deletion state in destination
	cleanupSource := false
	if name := p.Spec.Destination.GetDeploymentName(); name != "" {
		depl, err := depls.Get(context.Background(), name, meta.GetOptions{})
		if k8sutil.IsNotFound(err) {
			dr.log.Debug("Destination deployment is gone. Source cleanup enabled")
			cleanupSource = true
		} else if err != nil {
			dr.log.Err(err).Warn("Failed to get destinaton deployment")
			return errors.WithStack(err)
		} else if depl.GetDeletionTimestamp() != nil {
			dr.log.Debug("Destination deployment is being deleted. Source cleanup enabled")
			cleanupSource = true
		}
	}

	// Cleanup source or stop sync
	if cleanupSource {
		// Destination is gone, cleanup source
		/*sourceClient, err := dr.createSyncMasterClient(p.Spec.Source)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to create source client")
			return errors.WithStack(err)
		}*/
		//sourceClient.Master().C
		return errors.WithStack(errors.Newf("TODO"))
	} else {
		// Destination still exists, stop/abort sync
		destClient, err := dr.createSyncMasterClient(p.Spec.Destination)
		if err != nil {
			dr.log.Err(err).Warn("Failed to create destination client")
			return errors.WithStack(err)
		}
		req := client.CancelSynchronizationRequest{
			WaitTimeout:  time.Minute * 3,
			Force:        abort,
			ForceTimeout: time.Minute * 2,
		}
		dr.log.Bool("abort", abort).Debug("Stopping synchronization...")
		_, err = destClient.Master().CancelSynchronization(ctx, req)
		if err != nil && !client.IsPreconditionFailed(err) {
			dr.log.Err(err).Bool("abort", abort).Warn("Failed to stop synchronization")
			dr.status.CancelFailures++
			if err := dr.updateCRStatus(); err != nil {
				dr.log.Err(err).Warn("Failed to update status to reflect cancel-failures increment")
			}
			return errors.WithStack(err)
		}
		return nil
	}
}

// removeDeploymentReplicationFinalizers removes the given finalizers from the given DeploymentReplication.
func removeDeploymentReplicationFinalizers(crcli versioned.Interface, p *api.ArangoDeploymentReplication, finalizers []string, ignoreNotFound bool) error {
	repls := crcli.ReplicationV1().ArangoDeploymentReplications(p.GetNamespace())
	getFunc := func() (meta.Object, error) {
		result, err := repls.Get(context.Background(), p.GetName(), meta.GetOptions{})
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return result, nil
	}
	updateFunc := func(updated meta.Object) error {
		updatedRepl := updated.(*api.ArangoDeploymentReplication)
		result, err := repls.Update(context.Background(), updatedRepl, meta.UpdateOptions{})
		if err != nil {
			return errors.WithStack(err)
		}
		*p = *result
		return nil
	}
	if _, err := k8sutil.RemoveFinalizers(finalizers, getFunc, updateFunc, ignoreNotFound); err != nil {
		return errors.WithStack(err)
	}
	return nil
}
