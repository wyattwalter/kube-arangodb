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

package client

import (
	"net"
	"strconv"
	"sync"

	goHttp "net/http"

	driver "github.com/arangodb/go-driver"
	api "github.com/arangodb/kube-arangodb/pkg/apis/deployment/v1"
	"github.com/arangodb/kube-arangodb/pkg/apis/shared"
	"github.com/arangodb/kube-arangodb/pkg/deployment/reconciler/endpoints"
	"github.com/arangodb/kube-arangodb/pkg/deployment/reconciler/info"
	"github.com/arangodb/kube-arangodb/pkg/util/arangod/conn"
	"k8s.io/apimachinery/pkg/util/rand"
)

type Connections map[string]driver.Connection

func (c Connections) Keys() []string {
	z := make([]string, 0, len(c))

	for k := range c {
		z = append(z, k)
	}

	return z
}
func (c Connections) Filter(p func(c driver.Connection) bool) Connections {
	f := make(map[string]bool, len(c))

	var wg sync.WaitGroup

	for id := range c {
		wg.Add(1)
		go func(i string) {
			defer wg.Done()

			f[i] = p(c[i])
		}(id)
	}

	wg.Wait()

	r := make(Connections, len(f))

	for id := range f {
		if f[id] {
			r[id] = c[id]
		}
	}

	return r
}

func (c Connections) Random() (driver.Connection, bool) {
	keys := c.Keys()

	if len(keys) == 0 {
		return nil, false
	}

	return c[keys[rand.Intn(len(keys))]], true
}

type HTTPClient interface {
	Do(req *goHttp.Request) (*goHttp.Response, error)
}

type httpClient struct {
	client *goHttp.Client
	cache  *cache
}

func (h httpClient) Do(req *goHttp.Request) (*goHttp.Response, error) {
	auth := h.cache.factory.GetAuth()

	if auth != nil {
		a, err := auth()
		if err != nil {
			return nil, err
		}

		if a.Type() == driver.AuthenticationTypeRaw {
			if v := a.Get("value"); v != "" {
				req.Header.Add("Authorization", v)
			}
		}
	}

	return h.client.Do(req)
}

type Cache interface {
	Connection(hosts ...string) (driver.Connection, error)

	GetHTTPClient(mods ...func(cfg *goHttp.Transport)) HTTPClient

	GetConnection(group api.ServerGroup, id string) (driver.Connection, error)
	GetConnectionsForGroup(group api.ServerGroup) (Connections, error)
}

type CacheGen interface {
	endpoints.DeploymentEndpoints
	info.DeploymentInfoGetter
}

func NewClientCache(in CacheGen, factory conn.Factory) Cache {
	return &cache{
		in:      in,
		factory: factory,
	}
}

type cache struct {
	mutex sync.Mutex
	in    CacheGen

	factory conn.Factory
}

func (cc *cache) GetHTTPClient(mods ...func(cfg *goHttp.Transport)) HTTPClient {
	return httpClient{
		client: cc.factory.HTTPClient(mods...),
		cache:  cc,
	}
}

func (cc *cache) GetConnectionsForGroup(group api.ServerGroup) (Connections, error) {
	q := cc.in.GetStatusSnapshot().Members.AsListInGroup(group)

	r := make(Connections, len(q))
	for _, m := range q {
		c, err := cc.GetConnection(group, m.Member.ID)
		if err != nil {
			return nil, err
		}

		r[m.Member.ID] = c
	}

	return r, nil
}

func (cc *cache) GetConnection(group api.ServerGroup, id string) (driver.Connection, error) {
	cc.mutex.Lock()
	defer cc.mutex.Unlock()
	m, _, _ := cc.in.GetStatusSnapshot().Members.ElementByID(id)

	endpoint, err := cc.in.GenerateMemberEndpoint(group, m)
	if err != nil {
		return nil, err
	}

	return cc.factory.Connection(cc.extendHost(m.GetEndpoint(endpoint)))
}

func (cc *cache) Connection(hosts ...string) (driver.Connection, error) {
	return cc.factory.Connection(hosts...)
}

func (cc *cache) extendHost(host string) string {
	scheme := "http"
	if cc.in.GetSpec().TLS.IsSecure() {
		scheme = "https"
	}

	return scheme + "://" + net.JoinHostPort(host, strconv.Itoa(shared.ArangoPort))
}
