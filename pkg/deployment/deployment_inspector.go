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

package deployment

import (
	"context"
	"time"

	"github.com/arangodb/kube-arangodb/pkg/apis/deployment"
	api "github.com/arangodb/kube-arangodb/pkg/apis/deployment/v1"
	"github.com/arangodb/kube-arangodb/pkg/deployment/features"
	"github.com/arangodb/kube-arangodb/pkg/deployment/patch"
	"github.com/arangodb/kube-arangodb/pkg/metrics"
	"github.com/arangodb/kube-arangodb/pkg/upgrade"
	"github.com/arangodb/kube-arangodb/pkg/util"
	"github.com/arangodb/kube-arangodb/pkg/util/errors"
	"github.com/arangodb/kube-arangodb/pkg/util/globals"
	"github.com/arangodb/kube-arangodb/pkg/util/k8sutil"
	inspectorInterface "github.com/arangodb/kube-arangodb/pkg/util/k8sutil/inspector"
)

var (
	inspectDeploymentDurationGauges = metrics.MustRegisterGaugeVec(metricsComponent, "inspect_deployment_duration", "Amount of time taken by a single inspection of a deployment (in sec)", metrics.DeploymentName)
)

// inspectDeployment inspects the entire deployment, creates
// a plan to update if needed and inspects underlying resources.
// This function should be called when:
// - the deployment has changed
// - any of the underlying resources has changed
// - once in a while
// Returns the delay until this function should be called again.
func (d *Deployment) inspectDeployment(lastInterval util.Interval) util.Interval {
	start := time.Now()

	ctxReconciliation, cancelReconciliation := globals.GetGlobalTimeouts().Reconciliation().WithTimeout(context.Background())
	defer cancelReconciliation()
	defer func() {
		d.log.Trace("Inspect loop took %s", time.Since(start))
	}()

	nextInterval := lastInterval
	hasError := false

	deploymentName := d.GetName()
	defer metrics.SetDuration(inspectDeploymentDurationGauges.WithLabelValues(deploymentName), start)

	err := d.acs.CurrentClusterCache().Refresh(ctxReconciliation)
	if err != nil {
		d.log.Err(err).Error("Unable to get resources")
		return minInspectionInterval // Retry ASAP
	}

	// Check deployment still exists
	updated, err := d.acs.CurrentClusterCache().GetCurrentArangoDeployment()
	if k8sutil.IsNotFound(err) {
		// Deployment is gone
		d.log.Info("Deployment is gone")
		d.Stop()
		return nextInterval
	} else if updated != nil && updated.GetDeletionTimestamp() != nil {
		// Deployment is marked for deletion
		if err := d.runDeploymentFinalizers(ctxReconciliation, d.GetCachedStatus()); err != nil {
			hasError = true
			d.CreateEvent(k8sutil.NewErrorEvent("ArangoDeployment finalizer inspection failed", err, d.currentObject))
		}
	} else {
		// Check if maintenance annotation is set
		if updated != nil && updated.Annotations != nil {
			if v, ok := updated.Annotations[deployment.ArangoDeploymentPodMaintenanceAnnotation]; ok && v == "true" {
				// Disable checks if we will enter maintenance mode
				d.log.Str("deployment", deploymentName).Info("Deployment in maintenance mode")
				return nextInterval
			}
		}

		if ensureFinalizers(updated) {
			if err := d.ApplyPatch(ctxReconciliation, patch.ItemReplace(patch.NewPath("metadata", "finalizers"), updated.Finalizers)); err != nil {
				d.log.Err(err).Debug("Unable to set finalizers")
			}
		}

		if canProceed, changed, err := d.acceptNewSpec(ctxReconciliation, updated); err != nil {
			d.log.Err(err).Debug("Verification of deployment failed")

			if !canProceed {
				return minInspectionInterval // Retry ASAP
			}
		} else if changed {
			d.log.Info("Accepted new spec")
			// Notify cluster of desired server count
			if ci := d.clusterScalingIntegration; ci != nil {
				if c := d.currentObjectStatus; c != nil {
					if a := c.AcceptedSpec; a != nil {
						if c.Conditions.IsTrue(api.ConditionTypeUpToDate) {
							ci.SendUpdateToCluster(*a)
						}
					}
				}
			}
			return minInspectionInterval // Retry ASAP
		} else if !canProceed {
			d.log.Err(err).Error("Cannot proceed with reconciliation")
			return minInspectionInterval // Retry ASAP
		}

		// Ensure that status is up to date
		if !d.currentObjectStatus.Equal(updated.Status) {
			if err := d.updateCRStatus(ctxReconciliation, *d.currentObjectStatus); err != nil {
				d.log.Err(err).Error("Unable to refresh status")
				return minInspectionInterval // Retry ASAP
			}
		}

		d.currentObject = updated

		d.metrics.Deployment.Accepted = updated.Status.Conditions.IsTrue(api.ConditionTypeSpecAccepted)
		d.metrics.Deployment.UpToDate = updated.Status.Conditions.IsTrue(api.ConditionTypeUpToDate)

		// Is the deployment in failed state, if so, give up.
		if d.GetPhase() == api.DeploymentPhaseFailed {
			d.log.Debug("Deployment is in Failed state.")
			return nextInterval
		}

		d.GetMembersState().RefreshState(ctxReconciliation, updated.Status.Members.AsList())
		d.GetMembersState().Log(d.log)
		if err := d.WithStatusUpdateErr(ctxReconciliation, func(s *api.DeploymentStatus) (bool, error) {
			if changed, err := upgrade.RunUpgrade(*updated, s, d.GetCachedStatus()); err != nil {
				return false, err
			} else {
				return changed, nil
			}
		}); err != nil {
			d.CreateEvent(k8sutil.NewErrorEvent("Upgrade failed", err, d.currentObject))
			nextInterval = minInspectionInterval
			d.recentInspectionErrors++
			return nextInterval.ReduceTo(maxInspectionInterval)
		}

		inspectNextInterval, err := d.inspectDeploymentWithError(ctxReconciliation, nextInterval)
		if err != nil {
			if !errors.IsReconcile(err) {
				nextInterval = inspectNextInterval
				hasError = true

				d.CreateEvent(k8sutil.NewErrorEvent("Reconciliation failed", err, d.currentObject))
			} else {
				nextInterval = minInspectionInterval
			}
		}
	}

	// Update next interval (on errors)
	if hasError {
		if d.recentInspectionErrors == 0 {
			nextInterval = minInspectionInterval
			d.recentInspectionErrors++
		}
	} else {
		d.recentInspectionErrors = 0
	}
	return nextInterval.ReduceTo(maxInspectionInterval)
}

// inspectDeploymentWithError ensures that the deployment is in a valid state
func (d *Deployment) inspectDeploymentWithError(ctx context.Context, lastInterval util.Interval) (nextInterval util.Interval, inspectError error) {
	t := time.Now()

	defer func() {
		d.log.Trace("Reconciliation loop took %s", time.Since(t))
	}()

	// Ensure that spec and status checksum are same
	currentSpec := d.currentObject.Spec
	status := d.GetStatus()

	nextInterval = lastInterval
	inspectError = nil

	currentChecksum, err := currentSpec.Checksum()
	if err != nil {
		return minInspectionInterval, errors.Wrapf(err, "Calculation of spec failed")
	} else {
		condition, exists := status.Conditions.Get(api.ConditionTypeSpecAccepted)
		if v := status.AcceptedSpecVersion; (v == nil || currentChecksum != *v) && (!exists || condition.IsTrue()) {
			if err = d.updateConditionWithHash(ctx, api.ConditionTypeSpecAccepted, false, "Spec Changed", "Spec Object changed. Waiting to be accepted", currentChecksum); err != nil {
				return minInspectionInterval, errors.Wrapf(err, "Unable to update SpecAccepted condition")
			}

			return minInspectionInterval, nil // Retry ASAP
		} else if v != nil {
			if *v == currentChecksum && !condition.IsTrue() {
				if err = d.updateConditionWithHash(ctx, api.ConditionTypeSpecAccepted, true, "Spec Accepted", "Spec Object accepted", currentChecksum); err != nil {
					return minInspectionInterval, errors.Wrapf(err, "Unable to update SpecAccepted condition")
				}

				return minInspectionInterval, nil // Retry ASAP
			}
		}
	}

	if !status.Conditions.IsTrue(api.ConditionTypeSpecAccepted) {
		condition, exists := status.Conditions.Get(api.ConditionTypeUpToDate)
		if !exists || condition.IsTrue() {
			if err = d.updateConditionWithHash(ctx, api.ConditionTypeUpToDate, false, "Spec Changed", "Spec Object changed. Waiting until plan will be applied", currentChecksum); err != nil {
				return minInspectionInterval, errors.Wrapf(err, "Unable to update UpToDate condition")

			}

			return minInspectionInterval, nil // Retry ASAP
		}
	}

	if err := d.acs.Inspect(ctx, d.currentObject, d.deps.Client, d.GetCachedStatus()); err != nil {
		d.log.Err(err).Warn("Unable to handle ACS objects")
	}

	// Cleanup terminated pods on the beginning of loop
	if x, err := d.resources.CleanupTerminatedPods(ctx); err != nil {
		return minInspectionInterval, errors.Wrapf(err, "Pod cleanup failed")
	} else {
		nextInterval = nextInterval.ReduceTo(x)
	}

	if err := d.resources.EnsureLeader(ctx, d.GetCachedStatus()); err != nil {
		return minInspectionInterval, errors.Wrapf(err, "Creating leaders failed")
	}

	if err := d.resources.EnsureArangoMembers(ctx, d.GetCachedStatus()); err != nil {
		return minInspectionInterval, errors.Wrapf(err, "ArangoMember creation failed")
	}

	if err := d.resources.EnsureServices(ctx, d.GetCachedStatus()); err != nil {
		return minInspectionInterval, errors.Wrapf(err, "Service creation failed")
	}

	if err := d.resources.EnsureSecrets(ctx, d.GetCachedStatus()); err != nil {
		return minInspectionInterval, errors.Wrapf(err, "Secret creation failed")
	}

	// Inspect secret hashes
	if err := d.resources.ValidateSecretHashes(ctx, d.GetCachedStatus()); err != nil {
		return minInspectionInterval, errors.Wrapf(err, "Secret hash validation failed")
	}

	// Check for LicenseKeySecret
	if err := d.resources.ValidateLicenseKeySecret(d.GetCachedStatus()); err != nil {
		return minInspectionInterval, errors.Wrapf(err, "License Key Secret invalid")
	}

	// Is the deployment in a good state?
	if status.Conditions.IsTrue(api.ConditionTypeSecretsChanged) {
		return minInspectionInterval, errors.Newf("Secrets changed")
	}

	// Ensure we have image info
	if retrySoon, exists, err := d.ensureImages(ctx, d.currentObject, d.GetCachedStatus()); err != nil {
		return minInspectionInterval, errors.Wrapf(err, "Image detection failed")
	} else if retrySoon || !exists {
		return minInspectionInterval, nil
	}

	// Inspection of generated resources needed
	if x, err := d.resources.InspectPods(ctx, d.GetCachedStatus()); err != nil {
		return minInspectionInterval, errors.Wrapf(err, "Pod inspection failed")
	} else {
		nextInterval = nextInterval.ReduceTo(x)
	}

	if x, err := d.resources.InspectPVCs(ctx, d.GetCachedStatus()); err != nil {
		return minInspectionInterval, errors.Wrapf(err, "PVC inspection failed")
	} else {
		nextInterval = nextInterval.ReduceTo(x)
	}

	// Check members for resilience
	if err := d.resilience.CheckMemberFailure(ctx); err != nil {
		return minInspectionInterval, errors.Wrapf(err, "Member failure detection failed")
	}

	// Immediate actions
	if err := d.reconciler.CheckDeployment(ctx); err != nil {
		return minInspectionInterval, errors.Wrapf(err, "Reconciler immediate actions failed")
	}

	if interval, err := d.ensureResources(ctx, nextInterval, d.GetCachedStatus()); err != nil {
		return minInspectionInterval, errors.Wrapf(err, "Reconciler resource recreation failed")
	} else {
		nextInterval = interval
	}

	d.metrics.Agency.Fetches++
	if offset, err := d.RefreshAgencyCache(ctx); err != nil {
		d.metrics.Agency.Errors++
		d.log.Err(err).Error("Unable to refresh agency")
	} else {
		d.metrics.Agency.Index = offset
	}

	// Refresh maintenance lock
	d.refreshMaintenanceTTL(ctx)

	// Create scale/update plan
	if _, ok := d.currentObject.Annotations[deployment.ArangoDeploymentPlanCleanAnnotation]; ok {
		if err := d.ApplyPatch(ctx, patch.ItemRemove(patch.NewPath("metadata", "annotations", deployment.ArangoDeploymentPlanCleanAnnotation))); err != nil {
			return minInspectionInterval, errors.Wrapf(err, "Unable to create remove annotation patch")
		}

		if err := d.WithStatusUpdate(ctx, func(s *api.DeploymentStatus) bool {
			s.Plan = nil
			return true
		}); err != nil {
			return minInspectionInterval, errors.Wrapf(err, "Unable clean plan")
		}
	} else if err, updated := d.reconciler.CreatePlan(ctx); err != nil {
		return minInspectionInterval, errors.Wrapf(err, "Plan creation failed")
	} else if updated {
		d.log.Info("Plan generated, reconciling")
		return minInspectionInterval, nil
	}

	// Reachable state ensurer
	reachableConditionState := status.Conditions.Check(api.ConditionTypeReachable).Exists().IsTrue().Evaluate()
	if d.GetMembersState().State().IsReachable() {
		if !reachableConditionState {
			if err = d.updateConditionWithHash(ctx, api.ConditionTypeReachable, true, "ArangoDB is reachable", "", ""); err != nil {
				return minInspectionInterval, errors.Wrapf(err, "Unable to update Reachable condition")
			}
		}
	} else {
		if reachableConditionState {
			if err = d.updateConditionWithHash(ctx, api.ConditionTypeReachable, false, "ArangoDB is not reachable", "", ""); err != nil {
				return minInspectionInterval, errors.Wrapf(err, "Unable to update Reachable condition")
			}
		}
	}

	if v := status.AcceptedSpecVersion; v != nil && d.currentObject.Status.IsPlanEmpty() && status.AppliedVersion != *v {
		if err := d.WithStatusUpdate(ctx, func(s *api.DeploymentStatus) bool {
			s.AppliedVersion = *v
			return true
		}); err != nil {
			return minInspectionInterval, errors.Wrapf(err, "Unable to update UpToDate condition")
		}

		return minInspectionInterval, nil
	} else {
		isUpToDate, reason := d.isUpToDateStatus(status)

		if !isUpToDate && status.Conditions.IsTrue(api.ConditionTypeUpToDate) {
			if err = d.updateConditionWithHash(ctx, api.ConditionTypeUpToDate, false, reason, "There are pending operations in plan or members are in restart process", *v); err != nil {
				return minInspectionInterval, errors.Wrapf(err, "Unable to update UpToDate condition")
			}

			return minInspectionInterval, nil
		}

		if isUpToDate && !status.Conditions.IsTrue(api.ConditionTypeUpToDate) {
			if err = d.updateConditionWithHash(ctx, api.ConditionTypeUpToDate, true, "Spec is Up To Date", "Spec is Up To Date", *v); err != nil {
				return minInspectionInterval, errors.Wrapf(err, "Unable to update UpToDate condition")
			}

			return minInspectionInterval, nil
		}
	}

	// Execute current step of scale/update plan
	retrySoon, err := d.reconciler.ExecutePlan(ctx)
	if err != nil {
		return minInspectionInterval, errors.Wrapf(err, "Plan execution failed")
	}
	if retrySoon {
		nextInterval = minInspectionInterval
	}

	// Create access packages
	if err := d.createAccessPackages(ctx); err != nil {
		return minInspectionInterval, errors.Wrapf(err, "AccessPackage creation failed")
	}

	// Inspect deployment for synced members
	if health, ok := d.GetMembersState().Health(); ok {
		if err := d.resources.SyncMembersInCluster(ctx, health); err != nil {
			return minInspectionInterval, errors.Wrapf(err, "Removed member cleanup failed")
		}
	}

	// At the end of the inspect, we cleanup terminated pods.
	if x, err := d.resources.CleanupTerminatedPods(ctx); err != nil {
		return minInspectionInterval, errors.Wrapf(err, "Pod cleanup failed")
	} else {
		nextInterval = nextInterval.ReduceTo(x)
	}

	return
}

func (d *Deployment) isUpToDateStatus(status api.DeploymentStatus) (upToDate bool, reason string) {
	if !status.IsPlanEmpty() {
		return false, "Plan is not empty"
	}

	upToDate = true

	if v := status.AcceptedSpecVersion; v == nil || status.AppliedVersion != *v {
		upToDate = false
		reason = "Spec is not accepted"
		return
	}

	if !status.Conditions.Check(api.ConditionTypeSpecAccepted).Exists().IsTrue().Evaluate() {
		upToDate = false
		reason = "Spec is not accepted"
		return
	}

	if !status.Conditions.Check(api.ConditionTypeReachable).Exists().IsTrue().Evaluate() {
		upToDate = false
		return
	}

	for _, m := range status.Members.AsList() {
		member := m.Member
		if member.Conditions.IsTrue(api.ConditionTypeRestart) || member.Conditions.IsTrue(api.ConditionTypePendingRestart) {
			upToDate = false
			reason = "Pending restarts on members"
			return
		}
		if member.Conditions.IsTrue(api.ConditionTypePVCResizePending) {
			upToDate = false
			reason = "PVC is resizing"
			return
		}
	}

	return
}

func (d *Deployment) refreshMaintenanceTTL(ctx context.Context) {
	if d.GetSpec().Mode.Get() == api.DeploymentModeSingle {
		return
	}

	if !features.Maintenance().Enabled() {
		// Maintenance feature is not enabled
		return
	}

	agencyState, agencyOK := d.GetAgencyCache()
	if !agencyOK {
		return
	}

	status := d.GetStatus()

	condition, ok := status.Conditions.Get(api.ConditionTypeMaintenance)
	maintenance := agencyState.Supervision.Maintenance

	if !ok || !condition.IsTrue() {
		return
	}

	// Check GracePeriod
	if t, ok := maintenance.Time(); ok {
		if time.Until(t) < time.Hour-d.GetSpec().Timeouts.GetMaintenanceGracePeriod() {
			if err := d.SetAgencyMaintenanceMode(ctx, true); err != nil {
				return
			}
			d.log.Info("Refreshed maintenance lock")
		}
	} else {
		if condition.LastUpdateTime.Add(d.GetSpec().Timeouts.GetMaintenanceGracePeriod()).Before(time.Now()) {
			if err := d.SetAgencyMaintenanceMode(ctx, true); err != nil {
				return
			}
			d.log.Info("Refreshed maintenance lock")
		}
	}
}

// ensureResources creates all required resources for the deployment
func (d *Deployment) ensureResources(ctx context.Context, lastInterval util.Interval, cachedStatus inspectorInterface.Inspector) (util.Interval, error) {
	// Ensure all resources are created
	if d.haveServiceMonitorCRD {
		if err := d.resources.EnsureServiceMonitor(ctx); err != nil {
			return minInspectionInterval, errors.Wrapf(err, "Service monitor creation failed")
		}
	}

	if err := d.resources.EnsurePVCs(ctx, cachedStatus); err != nil {
		return minInspectionInterval, errors.Wrapf(err, "PVC creation failed")
	}

	if err := d.resources.EnsurePods(ctx, cachedStatus); err != nil {
		return minInspectionInterval, errors.Wrapf(err, "Pod creation failed")
	}

	if err := d.resources.EnsurePDBs(ctx); err != nil {
		return minInspectionInterval, errors.Wrapf(err, "PDB creation failed")
	}

	if err := d.resources.EnsureAnnotations(ctx, cachedStatus); err != nil {
		return minInspectionInterval, errors.Wrapf(err, "Annotation update failed")
	}

	if err := d.resources.EnsureLabels(ctx, cachedStatus); err != nil {
		return minInspectionInterval, errors.Wrapf(err, "Labels update failed")
	}

	return lastInterval, nil
}

// triggerInspection ensures that an inspection is run soon.
func (d *Deployment) triggerInspection() {
	d.inspectTrigger.Trigger()
}

// triggerCRDInspection ensures that an inspection is run soon.
func (d *Deployment) triggerCRDInspection() {
	d.inspectCRDTrigger.Trigger()
}

func (d *Deployment) updateConditionWithHash(ctx context.Context, conditionType api.ConditionType, status bool, reason, message, hash string) error {
	d.log.Str("condition", string(conditionType)).Bool("status", status).Str("reason", reason).Str("message", message).Str("hash", hash).Info("Updated condition")
	if err := d.WithStatusUpdate(ctx, func(s *api.DeploymentStatus) bool {
		return s.Conditions.UpdateWithHash(conditionType, status, reason, message, hash)
	}); err != nil {
		return errors.Wrapf(err, "Unable to update condition")
	}

	return nil
}
