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
	"encoding/json"
	"net/http"

	"sync"

	"github.com/arangodb/go-driver"
)

type agencyConfigResults map[string]*agencyConfigResult

type agencyConfigResult struct {
	config *agencyConfig
	err    error
	conn   driver.Connection
}

func getAgencyConfigResults(ctx context.Context, connections map[string]driver.Connection) agencyConfigResults {
	var wg sync.WaitGroup

	r := make(agencyConfigResults, len(connections))

	for k := range connections {
		r[k] = nil
	}

	for k := range connections {
		wg.Add(1)

		go func(key string) {
			defer wg.Done()

			r[key] = getAgencyConfigResult(ctx, connections[key])
		}(k)
	}

	wg.Wait()

	return r
}

func getAgencyConfigResult(ctx context.Context, conn driver.Connection) *agencyConfigResult {
	c, err := getAgencyConfig(ctx, conn)
	return &agencyConfigResult{
		config: c,
		err:    err,
		conn:   conn,
	}
}

func getAgencyConfig(ctx context.Context, conn driver.Connection) (*agencyConfig, error) {
	req, err := conn.NewRequest(http.MethodGet, "/_api/agency/config")
	if err != nil {
		return nil, err
	}

	var data []byte

	resp, err := conn.Do(driver.WithRawResponse(ctx, &data), req)
	if err != nil {
		return nil, err
	}

	if err := resp.CheckStatus(http.StatusOK); err != nil {
		return nil, err
	}

	var c agencyConfig

	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}

	return &c, nil
}

type agencyConfig struct {
	LeaderId *string `json:"leaderId,omitempty"`

	CommitIndex uint64 `json:"commitIndex"`

	Configuration struct {
		ID string `json:"id"`
	} `json:"configuration"`

	Pool   map[string]interface{} `json:"pool,omitempty"`
	Active []string               `json:"active,omitempty"`
}
