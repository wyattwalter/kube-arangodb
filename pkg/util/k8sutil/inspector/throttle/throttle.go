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

package throttle

import (
	"sync"
	"time"
)

type Inspector interface {
	GetThrottles() Components
}

func NewAlwaysThrottleComponents() Components {
	return NewThrottleComponents(0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
}

func NewThrottleComponents(acs, am, at, node, pvc, pod, pdb, secret, service, serviceAccount, sm, endpoints time.Duration) Components {
	return &throttleComponents{
		arangoClusterSynchronization: NewThrottle(acs),
		arangoMember:                 NewThrottle(am),
		arangoTask:                   NewThrottle(at),
		node:                         NewThrottle(node),
		persistentVolumeClaim:        NewThrottle(pvc),
		pod:                          NewThrottle(pod),
		podDisruptionBudget:          NewThrottle(pdb),
		secret:                       NewThrottle(secret),
		service:                      NewThrottle(service),
		serviceAccount:               NewThrottle(serviceAccount),
		serviceMonitor:               NewThrottle(sm),
		endpoints:                    NewThrottle(endpoints),
	}
}

type ComponentCount map[Component]int

type Component string

const (
	ArangoClusterSynchronization Component = "ArangoClusterSynchronization"
	ArangoMember                 Component = "ArangoMember"
	ArangoTask                   Component = "ArangoTask"
	Node                         Component = "Node"
	PersistentVolumeClaim        Component = "PersistentVolumeClaim"
	Pod                          Component = "Pod"
	PodDisruptionBudget          Component = "PodDisruptionBudget"
	Secret                       Component = "Secret"
	Service                      Component = "Service"
	ServiceAccount               Component = "ServiceAccount"
	ServiceMonitor               Component = "ServiceMonitor"
	Endpoints                    Component = "Endpoints"
)

func AllComponents() []Component {
	return []Component{
		ArangoClusterSynchronization,
		ArangoMember,
		ArangoTask,
		Node,
		PersistentVolumeClaim,
		Pod,
		PodDisruptionBudget,
		Secret,
		Service,
		ServiceAccount,
		ServiceMonitor,
		Endpoints,
	}
}

type Components interface {
	ArangoClusterSynchronization() Throttle
	ArangoMember() Throttle
	ArangoTask() Throttle
	Node() Throttle
	PersistentVolumeClaim() Throttle
	Pod() Throttle
	PodDisruptionBudget() Throttle
	Secret() Throttle
	Service() Throttle
	ServiceAccount() Throttle
	ServiceMonitor() Throttle
	Endpoints() Throttle

	Get(c Component) Throttle
	Invalidate(components ...Component)

	Counts() ComponentCount
	Copy() Components
}

type throttleComponents struct {
	arangoClusterSynchronization Throttle
	arangoMember                 Throttle
	arangoTask                   Throttle
	node                         Throttle
	persistentVolumeClaim        Throttle
	pod                          Throttle
	podDisruptionBudget          Throttle
	secret                       Throttle
	service                      Throttle
	serviceAccount               Throttle
	serviceMonitor               Throttle
	endpoints                    Throttle
}

func (t *throttleComponents) Endpoints() Throttle {
	return t.endpoints
}

func (t *throttleComponents) Counts() ComponentCount {
	z := ComponentCount{}

	for _, c := range AllComponents() {
		z[c] = t.Get(c).Count()
	}

	return z
}

func (t *throttleComponents) Invalidate(components ...Component) {
	for _, c := range components {
		t.Get(c).Invalidate()
	}
}

func (t *throttleComponents) Get(c Component) Throttle {
	if t == nil {
		return NewAlwaysThrottle()
	}
	switch c {
	case ArangoClusterSynchronization:
		return t.arangoClusterSynchronization
	case ArangoMember:
		return t.arangoMember
	case ArangoTask:
		return t.arangoTask
	case Node:
		return t.node
	case PersistentVolumeClaim:
		return t.persistentVolumeClaim
	case Pod:
		return t.pod
	case PodDisruptionBudget:
		return t.podDisruptionBudget
	case Secret:
		return t.secret
	case Service:
		return t.service
	case ServiceAccount:
		return t.serviceAccount
	case ServiceMonitor:
		return t.serviceMonitor
	case Endpoints:
		return t.endpoints
	default:
		return NewAlwaysThrottle()
	}
}

func (t *throttleComponents) Copy() Components {
	return &throttleComponents{
		arangoClusterSynchronization: t.arangoClusterSynchronization.Copy(),
		arangoMember:                 t.arangoMember.Copy(),
		arangoTask:                   t.arangoTask.Copy(),
		node:                         t.node.Copy(),
		persistentVolumeClaim:        t.persistentVolumeClaim.Copy(),
		pod:                          t.pod.Copy(),
		podDisruptionBudget:          t.podDisruptionBudget.Copy(),
		secret:                       t.secret.Copy(),
		service:                      t.service.Copy(),
		serviceAccount:               t.serviceAccount.Copy(),
		serviceMonitor:               t.serviceMonitor.Copy(),
		endpoints:                    t.endpoints.Copy(),
	}
}

func (t *throttleComponents) ArangoClusterSynchronization() Throttle {
	return t.arangoClusterSynchronization
}

func (t *throttleComponents) ArangoMember() Throttle {
	return t.arangoMember
}

func (t *throttleComponents) ArangoTask() Throttle {
	return t.arangoTask
}

func (t *throttleComponents) Node() Throttle {
	return t.node
}

func (t *throttleComponents) PersistentVolumeClaim() Throttle {
	return t.persistentVolumeClaim
}

func (t *throttleComponents) Pod() Throttle {
	return t.pod
}

func (t *throttleComponents) PodDisruptionBudget() Throttle {
	return t.podDisruptionBudget
}

func (t *throttleComponents) Secret() Throttle {
	return t.secret
}

func (t *throttleComponents) Service() Throttle {
	return t.service
}

func (t *throttleComponents) ServiceAccount() Throttle {
	return t.serviceAccount
}

func (t *throttleComponents) ServiceMonitor() Throttle {
	return t.serviceMonitor
}

type Throttle interface {
	Invalidate()
	Throttle() bool
	Delay()

	Copy() Throttle

	Count() int
}

func NewAlwaysThrottle() Throttle {
	return &alwaysThrottle{}
}

type alwaysThrottle struct {
	count int
}

func (a alwaysThrottle) Count() int {
	return a.count
}

func (a *alwaysThrottle) Copy() Throttle {
	return a
}

func (a alwaysThrottle) Invalidate() {

}

func (a alwaysThrottle) Throttle() bool {
	return true
}

func (a *alwaysThrottle) Delay() {
	a.count++
}

func NewThrottle(delay time.Duration) Throttle {
	if delay == 0 {
		return NewAlwaysThrottle()
	}
	return &throttle{
		delay: delay,
	}
}

type throttle struct {
	lock sync.Mutex

	delay time.Duration
	next  time.Time
	count int
}

func (t *throttle) Count() int {
	t.lock.Lock()
	defer t.lock.Unlock()

	return t.count
}

func (t *throttle) Copy() Throttle {
	return &throttle{
		delay: t.delay,
		next:  t.next,
		count: t.count,
	}
}

func (t *throttle) Delay() {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.next = time.Now().Add(t.delay)
	t.count++
}

func (t *throttle) Throttle() bool {
	t.lock.Lock()
	defer t.lock.Unlock()

	return t.next.IsZero() || t.next.Before(time.Now())
}

func (t *throttle) Invalidate() {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.next = time.UnixMilli(0)
}
