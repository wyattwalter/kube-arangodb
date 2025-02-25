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

package resources

import (
	"os"
	"path/filepath"

	core "k8s.io/api/core/v1"

	api "github.com/arangodb/kube-arangodb/pkg/apis/deployment/v1"
	"github.com/arangodb/kube-arangodb/pkg/apis/shared"
	"github.com/arangodb/kube-arangodb/pkg/util/errors"
	"github.com/arangodb/kube-arangodb/pkg/util/k8sutil"
	"github.com/arangodb/kube-arangodb/pkg/util/k8sutil/probes"
)

// ArangodbInternalExporterContainer creates metrics container based on internal exporter
func ArangodbInternalExporterContainer(image string, args []string, livenessProbe *probes.HTTPProbeConfig,
	resources core.ResourceRequirements, securityContext *core.SecurityContext,
	spec api.DeploymentSpec) (core.Container, error) {

	binaryPath, err := os.Executable()
	if err != nil {
		return core.Container{}, errors.WithStack(err)
	}
	exePath := filepath.Join(k8sutil.LifecycleVolumeMountDir, filepath.Base(binaryPath))

	c := core.Container{
		Name:    shared.ExporterContainerName,
		Image:   image,
		Command: append([]string{exePath, "exporter"}, args...),
		Ports: []core.ContainerPort{
			{
				Name:          "exporter",
				ContainerPort: int32(spec.Metrics.GetPort()),
				Protocol:      core.ProtocolTCP,
			},
		},
		Resources:       k8sutil.ExtractPodResourceRequirement(resources),
		ImagePullPolicy: core.PullIfNotPresent,
		SecurityContext: securityContext,
		VolumeMounts:    []core.VolumeMount{k8sutil.LifecycleVolumeMount()},
	}

	if livenessProbe != nil {
		c.LivenessProbe = livenessProbe.Create()
	}

	return c, nil
}
