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
// Adam Janikowski
//

package conn

import (
	goHttp "net/http"

	"github.com/arangodb/go-driver"
	"github.com/arangodb/go-driver/http"
)

type Auth func() (driver.Authentication, error)
type Config func() *goHttp.Transport

type Factory interface {
	HTTPClient(mods ...func(cfg *goHttp.Transport)) *goHttp.Client

	Connection(hosts ...string) (driver.Connection, error)

	GetAuth() Auth
}

func NewFactory(auth Auth, config Config) Factory {
	return &factory{
		auth:   auth,
		config: config,
	}
}

type factory struct {
	auth   Auth
	config Config
}

func (f factory) HTTPClient(mods ...func(cfg *goHttp.Transport)) *goHttp.Client {
	t := f.config()

	for _, m := range mods {
		if m != nil {
			m(t)
		}
	}

	return &goHttp.Client{
		Transport: t,
	}
}

func (f factory) GetAuth() Auth {
	return f.auth
}

func (f factory) Connection(hosts ...string) (driver.Connection, error) {
	transport := f.config()

	cfg := http.ConnectionConfig{
		Endpoints:          hosts,
		Transport:          transport,
		DontFollowRedirect: true,
	}

	conn, err := http.NewConnection(cfg)
	if err != nil {
		return nil, err
	}

	return WrapAuthentication(conn, f.auth), nil
}
