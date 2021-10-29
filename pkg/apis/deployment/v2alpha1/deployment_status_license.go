//
// DISCLAIMER
//
// Copyright 2016-2021 ArangoDB GmbH, Cologne, Germany
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

package v2alpha1

type DeploymentStatusLicense struct {
	V2 *DeploymentStatusLicenseDetails `json:"v2,omitempty"`
}

func (a *DeploymentStatusLicense) Equal(b *DeploymentStatusLicense) bool {
	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	return a.V2.Equal(b.V2)
}

type DeploymentStatusLicenseDetails struct {
	Hash      string `json:"hash,omitempty"`
	Succeeded bool   `json:"succedeed"`
	Error     string `json:"error,omitempty"`
}

func (a *DeploymentStatusLicenseDetails) Equal(b *DeploymentStatusLicenseDetails) bool {
	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	return a.Hash == b.Hash && a.Succeeded == b.Succeeded && a.Error == b.Error
}
