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

import (
	"math"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
)

type TopologyStatus struct {
	ID types.UID `json:"id"`

	Size int `json:"size,omitempty"`

	Zones TopologyStatusZones `json:"zones,omitempty"`

	Label string `json:"label,omitempty"`
}

func (in *TopologyStatus) Equal(b *TopologyStatus) bool {
	if in == nil && b == nil {
		return true
	}

	if in == nil || b == nil {
		return false
	}

	return in.ID == b.ID &&
		in.Size == b.Size &&
		in.Label == b.Label &&
		in.Zones.Equal(b.Zones)
}

func (in *TopologyStatus) GetLeastUsedZone(group ServerGroup) int {
	if in == nil {
		return -1
	}

	r, m := -1, math.MaxInt64

	for i, z := range in.Zones {
		if n, ok := z.Members[group.AsRoleAbbreviated()]; ok {
			if v := len(n); v < m {
				r, m = i, v
			}
		} else {
			if v := 0; v < m {
				r, m = i, v
			}
		}
	}

	return r
}

func (in *TopologyStatus) RegisterTopologyLabel(zone int, label string) bool {
	if in == nil {
		return false
	}

	if zone < 0 || zone >= in.Size {
		return false
	}

	if in.Zones[zone].Labels.Contains(label) {
		return false
	}

	in.Zones[zone].Labels = in.Zones[zone].Labels.Add(label).Sort()

	return true
}

func (in *TopologyStatus) RemoveMember(group ServerGroup, id string) bool {
	if in == nil {
		return false
	}

	for _, zone := range in.Zones {
		if zone.RemoveMember(group, id) {
			return true
		}
	}

	return false
}

func (in *TopologyStatus) IsTopologyOwned(m *TopologyMemberStatus) bool {
	if in == nil {
		return false
	}

	if m == nil {
		return false
	}

	return in.ID == m.ID
}

func (in *TopologyStatus) Enabled() bool {
	return in != nil
}

type TopologyStatusZones []TopologyStatusZone

func (in TopologyStatusZones) Equal(b TopologyStatusZones) bool {
	if len(in) != len(b) {
		return false
	}

	for i := range in {
		if !in[i].Equal(b[i]) {
			return false
		}
	}

	return true
}

type TopologyStatusZoneMembers map[string]List

func (in TopologyStatusZoneMembers) Equal(b TopologyStatusZoneMembers) bool {
	if len(in) != len(b) {
		return false
	}

	for i, av := range in {
		if bv, ok := b[i]; !ok {
			return false
		} else {
			if !av.Equal(bv) {
				return false
			}
		}
	}

	return true
}

type TopologyStatusZone struct {
	ID int `json:"id"`

	Labels List `json:"labels,omitempty"`

	Members TopologyStatusZoneMembers `json:"members,omitempty"`
}

func (in TopologyStatusZone) Equal(b TopologyStatusZone) bool {
	return in.ID == b.ID && in.Labels.Equal(b.Labels) && in.Members.Equal(b.Members)
}

func (in *TopologyStatusZone) AddMember(group ServerGroup, id string) {
	if in.Members == nil {
		in.Members = TopologyStatusZoneMembers{}
	}

	in.Members[group.AsRoleAbbreviated()] = in.Members[group.AsRoleAbbreviated()].Add(id).Sort()
}

func (in *TopologyStatusZone) RemoveMember(group ServerGroup, id string) bool {
	if in == nil {
		return false
	}
	if in.Members == nil {
		return false
	}
	if !in.Members[group.AsRoleAbbreviated()].Contains(id) {
		return false
	}
	in.Members[group.AsRoleAbbreviated()] = in.Members[group.AsRoleAbbreviated()].Remove(id)
	return true
}

func (in *TopologyStatusZone) Get(group ServerGroup) List {
	if in == nil {
		return nil
	}

	if v, ok := in.Members[group.AsRoleAbbreviated()]; ok {
		return v
	} else {
		return nil
	}
}

func NewTopologyStatus(spec *TopologySpec) *TopologyStatus {
	if spec == nil {
		return nil
	}
	zones := make(TopologyStatusZones, spec.Zones)

	for i := 0; i < spec.Zones; i++ {
		zones[i] = TopologyStatusZone{ID: i}
	}

	return &TopologyStatus{
		ID:    uuid.NewUUID(),
		Size:  spec.Zones,
		Zones: zones,
		Label: spec.GetLabel(),
	}
}
