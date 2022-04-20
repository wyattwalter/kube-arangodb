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
	"github.com/arangodb/kube-arangodb/pkg/handlers/utils"
	"github.com/arangodb/kube-arangodb/pkg/util/errors"
	"github.com/arangodb/kube-arangodb/pkg/util/globals"
)

type Health map[string]bool

func (h Health) Healthy(except ...string) int {
	z := 0

	ex := utils.StringList(except)

	for n, v := range h {
		if !ex.Has(n) && v {
			z++
		}
	}

	return z
}

type Set interface {
	SetMembers(status api.DeploymentStatus) error
	Leader() (string, uint64, driver.Connection, bool)
	Agent(id string) (driver.Connection, bool)
	Health() Health
	Size() int
}

type agentSetResult struct {
	id     string
	result agencyConfig
	conn   driver.Connection
}

type agentSet struct {
	lock sync.Mutex

	cache client.Cache

	clients map[string]driver.Connection
	health  map[string]bool

	result *agentSetResult
}

func (a *agentSet) Size() int {
	a.lock.Lock()
	defer a.lock.Unlock()

	return len(a.clients)
}

func (a *agentSet) Health() Health {
	a.lock.Lock()
	defer a.lock.Unlock()

	return a.health
}

func (a *agentSet) Agent(id string) (driver.Connection, bool) {
	a.lock.Lock()
	defer a.lock.Unlock()

	c, ok := a.clients[id]
	return c, ok
}

func (a *agentSet) Leader() (string, uint64, driver.Connection, bool) {
	a.lock.Lock()
	defer a.lock.Unlock()

	z := a.result

	if z == nil {
		return "", 0, nil, false
	}

	return z.id, z.result.CommitIndex, z.conn, true
}

func (a *agentSet) refresh(ctx context.Context) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	var result *agentSetResult
	health := make(map[string]bool, len(a.clients))
	defer func() {
		a.result = result
		a.health = health
	}()

	nCtx, cancel := globals.GetGlobals().Timeouts().ArangoD().WithTimeout(ctx)
	defer cancel()

	r := getAgencyConfigResults(nCtx, a.clients)

	var leader *string

	for _, v := range r {
		if v.err != nil {
			continue
		}

		if cfg := v.config; cfg != nil {
			if l := cfg.LeaderId; l != nil {
				leader = l
				break
			}
		}
	}

	if leader == nil {
		return errors.Newf("NoLeader in Agency")
	}

	res, ok := r[*leader]
	if !ok {
		return errors.Newf("Leader not in result list")
	}

	if err := res.err; err != nil {
		return errors.Wrap(err, "Error while fetching from agency")
	}

	cfg := res.config
	if cfg == nil {
		return errors.Newf("Config result is missing")
	}

	health[*leader] = true

	for _, z := range cfg.Active {
		q, ok := r[z]
		if ok && q.err == nil {
			health[z] = true
		}
	}

	result = &agentSetResult{
		id:     *leader,
		result: *cfg,
	}

	return nil
}

func (a *agentSet) SetMembers(status api.DeploymentStatus) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	if agency := status.Agency; agency != nil {
		for _, id := range agency.IDs {
			if _, ok := a.clients[id]; !ok {
				c, err := a.cache.GetConnection(api.ServerGroupAgents, id)
				if err != nil {
					return err
				}

				a.clients[id] = c
			}
		}
	}

	return nil
}
