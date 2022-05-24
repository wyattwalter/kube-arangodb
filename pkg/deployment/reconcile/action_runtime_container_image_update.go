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
	core "k8s.io/api/core/v1"
	"time"

	"github.com/arangodb/kube-arangodb/pkg/deployment/rotation"
	"github.com/arangodb/kube-arangodb/pkg/util/k8sutil"
	"github.com/rs/zerolog"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/arangodb/kube-arangodb/pkg/apis/deployment/v1"
	"github.com/arangodb/kube-arangodb/pkg/util/errors"
)

func init() {
	registerAction(api.ActionTypeRuntimeContainerImageUpdate, runtimeContainerImageUpdate, defaultTimeout)
}

func runtimeContainerImageUpdate(log zerolog.Logger, action api.Action, actionCtx ActionContext) Action {
	a := &actionRuntimeContainerImageUpdate{}

	a.actionImpl = newBaseActionImplDefRef(log, action, actionCtx)

	return a
}

var _ ActionPost = &actionRuntimeContainerImageUpdate{}

type actionRuntimeContainerImageUpdate struct {
	// actionImpl implement timeout and member id functions
	actionImpl
}

func (a actionRuntimeContainerImageUpdate) Post(ctx context.Context) error {
	a.log.Info().Msgf("Updating container image")
	m, ok := a.actionCtx.GetMemberStatusByID(a.action.MemberID)
	if !ok {
		a.log.Info().Msg("member is gone already")
		return nil
	}

	name, image, ok := a.getContainerDetails()
	if !ok {
		a.log.Info().Msg("Unable to find container details")
		return nil
	}

	member, ok := a.actionCtx.ACS().CurrentClusterCache().ArangoMember().V1().GetSimple(m.ArangoMemberName(a.actionCtx.GetName(), a.action.Group))
	if !ok {
		err := errors.Newf("ArangoMember not found")
		a.log.Error().Err(err).Msg("ArangoMember not found")
		return err
	}

	return a.actionCtx.WithCurrentArangoMember(member.GetName()).UpdateStatus(ctx, func(obj *api.ArangoMember, s *api.ArangoMemberStatus) bool {
		if obj.Spec.Template == nil || s.Template == nil ||
			obj.Spec.Template.PodSpec == nil || s.Template.PodSpec == nil {
			a.log.Info().Msgf("Nil Member definition")
			return false
		}

		if len(obj.Spec.Template.PodSpec.Spec.Containers) != len(s.Template.PodSpec.Spec.Containers) {
			a.log.Info().Msgf("Invalid size of containers")
			return false
		}

		for id := range obj.Spec.Template.PodSpec.Spec.Containers {
			if obj.Spec.Template.PodSpec.Spec.Containers[id].Name == name {
				if s.Template.PodSpec.Spec.Containers[id].Name != name {
					a.log.Info().Msgf("Invalid order of containers")
					return false
				}

				if obj.Spec.Template.PodSpec.Spec.Containers[id].Image != image {
					a.log.Info().Str("got", obj.Spec.Template.PodSpec.Spec.Containers[id].Image).Str("expected", image).Msgf("Invalid spec image of container")
					return false
				}

				if s.Template.PodSpec.Spec.Containers[id].Image != image {
					s.Template.PodSpec.Spec.Containers[id].Image = image
					return true
				}
				return false
			}
		}
		return false
	})
}

func (a actionRuntimeContainerImageUpdate) getContainerDetails() (string, string, bool) {
	container, ok := a.action.GetParam(rotation.ContainerName)
	if !ok {
		return "", "", false
	}

	image, ok := a.action.GetParam(rotation.ContainerImage)
	if !ok {
		return "", "", false
	}

	return container, image, true
}

// Start starts the action for changing conditions on the provided member.
func (a actionRuntimeContainerImageUpdate) Start(ctx context.Context) (bool, error) {
	m, ok := a.actionCtx.GetMemberStatusByID(a.action.MemberID)
	if !ok {
		a.log.Info().Msg("member is gone already")
		return true, nil
	}

	cache, ok := a.actionCtx.ACS().ClusterCache(m.ClusterID)
	if !ok {
		return true, errors.Newf("Client is not ready")
	}

	name, image, ok := a.getContainerDetails()
	if !ok {
		a.log.Info().Msg("Unable to find container details")
		return true, nil
	}

	if !m.Phase.IsReady() {
		a.log.Info().Msg("Member is not ready, unable to run update operation")
		return true, nil
	}

	member, ok := a.actionCtx.ACS().CurrentClusterCache().ArangoMember().V1().GetSimple(m.ArangoMemberName(a.actionCtx.GetName(), a.action.Group))
	if !ok {
		err := errors.Newf("ArangoMember not found")
		a.log.Error().Err(err).Msg("ArangoMember not found")
		return false, err
	}

	pod, ok := cache.Pod().V1().GetSimple(m.PodName)
	if !ok {
		a.log.Info().Msg("pod is not present")
		return true, nil
	}

	if member.Spec.Template == nil || member.Spec.Template.PodSpec == nil {
		a.log.Info().Msg("pod spec is not present")
		return true, nil
	}

	if member.Status.Template == nil || member.Status.Template.PodSpec == nil {
		a.log.Info().Msg("pod status is not present")
		return true, nil
	}

	if len(pod.Spec.Containers) != len(member.Spec.Template.PodSpec.Spec.Containers) {
		a.log.Info().Msg("spec container count is not equal")
		return true, nil
	}

	if len(pod.Spec.Containers) != len(member.Status.Template.PodSpec.Spec.Containers) {
		a.log.Info().Msg("status container count is not equal")
		return true, nil
	}

	spec := member.Spec.Template.PodSpec
	status := member.Status.Template.PodSpec

	for id := range pod.Spec.Containers {
		if pod.Spec.Containers[id].Name != spec.Spec.Containers[id].Name ||
			pod.Spec.Containers[id].Name != status.Spec.Containers[id].Name ||
			pod.Spec.Containers[id].Name != name {
			continue
		}

		if pod.Spec.Containers[id].Image != image {
			// Update pod image
			pod.Spec.Containers[id].Image = image

			if _, err := a.actionCtx.PodsModInterface().Update(ctx, pod, v1.UpdateOptions{}); err != nil {
				return true, err
			}

			// Start wait action
			return false, nil
		}

		return true, nil
	}

	return true, nil
}

func (a actionRuntimeContainerImageUpdate) CheckProgress(ctx context.Context) (bool, bool, error) {
	a.log.Info().Msgf("Update Progress")
	m, ok := a.actionCtx.GetMemberStatusByID(a.action.MemberID)
	if !ok {
		a.log.Info().Msg("member is gone already")
		return true, false, nil
	}

	cache, ok := a.actionCtx.ACS().ClusterCache(m.ClusterID)
	if !ok {
		a.log.Info().Msg("Cluster is not ready")
		return false, false, nil
	}

	pod, ok := cache.Pod().V1().GetSimple(m.PodName)
	if !ok {
		a.log.Info().Msg("pod is not present")
		return true, false, nil
	}

	name, image, ok := a.getContainerDetails()
	if !ok {
		a.log.Info().Msg("Unable to find container details")
		return true, false, nil
	}

	cspec, ok := k8sutil.GetContainerByName(pod, name)
	if !ok {
		a.log.Info().Msg("Unable to find container spec")
		return true, false, nil
	}

	cstatus, ok := k8sutil.GetContainerStatusByName(pod, name)
	if !ok {
		a.log.Info().Msg("Unable to find container status")
		return true, false, nil
	}

	if cspec.Image != image {
		a.log.Info().Msg("Image changed")
		return true, false, nil
	}

	if s := cstatus.State.Terminated; s != nil {
		// We are in terminated state
		// Image is changed after start
		if cspec.Image != cstatus.Image {
			// Image not yet updated, retry soon
			return false, false, nil
		}

		// Pod won't get up and running
		return true, false, errors.Newf("Container %s failed during image replacement: (%d) %s: %s", name, s.ExitCode, s.Reason, s.Message)
	} else if s := cstatus.State.Waiting; s != nil {
		if pod.Spec.RestartPolicy == core.RestartPolicyAlways {
			lastTermination := cstatus.LastTerminationState.Terminated
			if lastTermination != nil {
				allowedRestartPeriod := time.Now().Add(time.Second * -20)
				if lastTermination.FinishedAt.Time.Before(allowedRestartPeriod) {
					return true, false, errors.Newf("Container %s continuously failed during image replacement: (%d) %s: %s", name, lastTermination.ExitCode, lastTermination.Reason, lastTermination.Message)
				} else {
					a.log.Debug().Str("pod-name", pod.GetName()).Msg("pod is restarting - we are not marking it as terminated yet..")
				}
			}
		}

		// Pod is still pulling image or pending for pod start
		return false, false, nil
	} else if s := cstatus.State.Running; s != nil {
		// Image is changed after restart
		if cspec.Image != cstatus.Image {
			// Image not yet updated, retry soon
			return false, false, nil
		}

		return true, false, nil
	} else {
		// Unknown state
		return false, false, nil
	}
}
