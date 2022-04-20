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

package conn

import (
	"context"
	"net/http"
	"sync"

	"github.com/arangodb/go-driver"
	"github.com/arangodb/kube-arangodb/pkg/util/errors"
)

func WrapAuthentication(conn driver.Connection, f Auth) driver.Connection {
	return &authenticate{
		base: conn,
		conn: conn,
		auth: f,
	}
}

type authenticate struct {
	lock sync.Mutex

	base driver.Connection

	conn driver.Connection

	auth Auth
}

func (a *authenticate) Do(ctx context.Context, req driver.Request) (driver.Response, error) {
	resp, err := a.conn.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != http.StatusUnauthorized {
		return resp, err
	}

	_, err = a.Reauthenticate()
	if err != nil {
		return nil, err
	}

	return a.conn.Do(ctx, req)
}

func (a *authenticate) Unmarshal(data driver.RawObject, result interface{}) error {
	return a.conn.Unmarshal(data, result)
}

func (a *authenticate) Endpoints() []string {
	return a.conn.Endpoints()
}

func (a *authenticate) UpdateEndpoints(endpoints []string) error {
	return a.conn.UpdateEndpoints(endpoints)
}

func (a *authenticate) Reauthenticate() (driver.Connection, error) {
	a.lock.Lock()
	defer a.lock.Unlock()

	auth, err := a.auth()
	if err != nil {
		return nil, errors.Wrap(err, "Unable to refresh auth")
	}

	c, err := a.base.SetAuthentication(auth)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to set auth")
	}

	a.conn = c

	return a, nil
}

func (a *authenticate) SetAuthentication(authentication driver.Authentication) (driver.Connection, error) {
	z, err := a.base.SetAuthentication(authentication)
	if err != nil {
		return nil, err
	}

	a.conn = z

	return a, nil
}

func (a *authenticate) Protocols() driver.ProtocolSet {
	return a.conn.Protocols()
}

func (a *authenticate) NewRequest(method, path string) (driver.Request, error) {
	return a.conn.NewRequest(method, path)
}
