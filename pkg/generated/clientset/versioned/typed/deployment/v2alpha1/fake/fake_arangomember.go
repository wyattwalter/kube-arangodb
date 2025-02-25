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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v2alpha1 "github.com/arangodb/kube-arangodb/pkg/apis/deployment/v2alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeArangoMembers implements ArangoMemberInterface
type FakeArangoMembers struct {
	Fake *FakeDatabaseV2alpha1
	ns   string
}

var arangomembersResource = schema.GroupVersionResource{Group: "database.arangodb.com", Version: "v2alpha1", Resource: "arangomembers"}

var arangomembersKind = schema.GroupVersionKind{Group: "database.arangodb.com", Version: "v2alpha1", Kind: "ArangoMember"}

// Get takes name of the arangoMember, and returns the corresponding arangoMember object, and an error if there is any.
func (c *FakeArangoMembers) Get(ctx context.Context, name string, options v1.GetOptions) (result *v2alpha1.ArangoMember, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(arangomembersResource, c.ns, name), &v2alpha1.ArangoMember{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v2alpha1.ArangoMember), err
}

// List takes label and field selectors, and returns the list of ArangoMembers that match those selectors.
func (c *FakeArangoMembers) List(ctx context.Context, opts v1.ListOptions) (result *v2alpha1.ArangoMemberList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(arangomembersResource, arangomembersKind, c.ns, opts), &v2alpha1.ArangoMemberList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v2alpha1.ArangoMemberList{ListMeta: obj.(*v2alpha1.ArangoMemberList).ListMeta}
	for _, item := range obj.(*v2alpha1.ArangoMemberList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested arangoMembers.
func (c *FakeArangoMembers) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(arangomembersResource, c.ns, opts))

}

// Create takes the representation of a arangoMember and creates it.  Returns the server's representation of the arangoMember, and an error, if there is any.
func (c *FakeArangoMembers) Create(ctx context.Context, arangoMember *v2alpha1.ArangoMember, opts v1.CreateOptions) (result *v2alpha1.ArangoMember, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(arangomembersResource, c.ns, arangoMember), &v2alpha1.ArangoMember{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v2alpha1.ArangoMember), err
}

// Update takes the representation of a arangoMember and updates it. Returns the server's representation of the arangoMember, and an error, if there is any.
func (c *FakeArangoMembers) Update(ctx context.Context, arangoMember *v2alpha1.ArangoMember, opts v1.UpdateOptions) (result *v2alpha1.ArangoMember, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(arangomembersResource, c.ns, arangoMember), &v2alpha1.ArangoMember{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v2alpha1.ArangoMember), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeArangoMembers) UpdateStatus(ctx context.Context, arangoMember *v2alpha1.ArangoMember, opts v1.UpdateOptions) (*v2alpha1.ArangoMember, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(arangomembersResource, "status", c.ns, arangoMember), &v2alpha1.ArangoMember{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v2alpha1.ArangoMember), err
}

// Delete takes name of the arangoMember and deletes it. Returns an error if one occurs.
func (c *FakeArangoMembers) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(arangomembersResource, c.ns, name), &v2alpha1.ArangoMember{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeArangoMembers) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(arangomembersResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v2alpha1.ArangoMemberList{})
	return err
}

// Patch applies the patch and returns the patched arangoMember.
func (c *FakeArangoMembers) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v2alpha1.ArangoMember, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(arangomembersResource, c.ns, name, pt, data, subresources...), &v2alpha1.ArangoMember{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v2alpha1.ArangoMember), err
}
