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

package agency

import (
	"context"
	"sync"

	"github.com/arangodb/go-driver"
	api "github.com/arangodb/kube-arangodb/pkg/apis/deployment/v1"
	"github.com/arangodb/kube-arangodb/pkg/deployment/client"
	"github.com/pkg/errors"
)

type Cache interface {
	Reload(ctx context.Context) (uint64, error)
	Data() (State, bool)

	AgentSet() Set

	CommitIndex() uint64
}

func NewCache(c client.Cache, mode *api.DeploymentMode) Cache {
	if mode.Get() == api.DeploymentModeSingle {
		return NewSingleCache()
	}

	return NewAgencyCache(c)
}

func NewAgencyCache(c client.Cache) Cache {
	return &cache{
		set: &agentSet{
			cache:   c,
			clients: map[string]driver.Connection{},
			result:  nil,
		},
	}
}

func NewSingleCache() Cache {
	return &cacheSingle{}
}

type cacheSingleSet struct {
}

func (c cacheSingleSet) SetMembers(status api.DeploymentStatus) error {
	return nil
}

func (c cacheSingleSet) Health() Health {
	return nil
}

func (c cacheSingleSet) Size() int {
	return 0
}

func (c cacheSingleSet) Leader() (string, uint64, driver.Connection, bool) {
	return "", 0, nil, false
}

func (c cacheSingleSet) Agent(id string) (driver.Connection, bool) {
	return nil, false
}

type cacheSingle struct {
}

func (c cacheSingle) AgentSet() Set {
	return cacheSingleSet{}
}

func (c cacheSingle) Leader() (string, bool) {
	return "", true
}

func (c cacheSingle) CommitIndex() uint64 {
	return 0
}

func (c cacheSingle) Reload(ctx context.Context) (uint64, error) {
	return 0, nil
}

func (c cacheSingle) Data() (State, bool) {
	return State{}, true
}

type cache struct {
	lock sync.Mutex

	valid bool

	commitIndex uint64

	data State

	set *agentSet
}

func (c *cache) AgentSet() Set {
	return c.set
}

func (c *cache) CommitIndex() uint64 {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.commitIndex
}

func (c *cache) Data() (State, bool) {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.data, c.valid
}

func (c *cache) Reload(ctx context.Context) (uint64, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if err := c.set.refresh(ctx); err != nil {
		return 0, err
	}

	_, commitIndex, conn, ok := c.set.Leader()

	if !ok {
		return 0, errors.New("Set did not refresh properly")
	}

	if commitIndex == c.commitIndex && c.valid {
		// We are on same index, nothing to do
		return commitIndex, nil
	}

	if data, err := loadState(ctx, conn); err != nil {
		c.valid = false
		return commitIndex, err
	} else {
		c.data = data
		c.valid = true
		c.commitIndex = commitIndex
		return commitIndex, nil
	}
}
