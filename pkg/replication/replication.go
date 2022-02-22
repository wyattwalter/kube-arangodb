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
	"github.com/rs/zerolog"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"

	"github.com/arangodb/kube-arangodb/pkg/generated/clientset/versioned"
)

// Config holds configuration settings for a DeploymentReplication
type Config struct {
	Namespace string
}

// Dependencies holds dependent services for a DeploymentReplication
type Dependencies struct {
	Log           zerolog.Logger
	KubeCli       kubernetes.Interface
	CRCli         versioned.Interface
	EventRecorder record.EventRecorder
}
