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

type StateTarget struct {
	HotBackup StateTargetHotBackup `json:"HotBackup,omitempty"`
	ToDo      Jobs                 `json:"ToDo,omitempty"`
	Pending   Jobs                 `json:"Pending,omitempty"`
	Finished  Jobs                 `json:"Finished,omitempty"`
	Failed    Jobs                 `json:"Failed,omitempty"`
}

type StateTargetHotBackup struct {
	Create StateExists `json:"Create,omitempty"`
}

func (s StateTarget) GetJobStatus(i string) (JobStatus, Job) {
	id := JobID(i)

	if v, ok := s.ToDo[id]; ok {
		return JobStatusToDo, v
	}
	if v, ok := s.Pending[id]; ok {
		return JobStatusPending, v
	}
	if v, ok := s.Finished[id]; ok {
		return JobStatusFinished, v
	}
	if v, ok := s.Failed[id]; ok {
		return JobStatusFailed, v
	}

	return JobStatusUnknown, Job{}
}
