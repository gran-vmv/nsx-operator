/* Copyright © 2024 VMware, Inc. All Rights Reserved.
   SPDX-License-Identifier: Apache-2.0 */

// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	"time"

	v1alpha1 "github.com/vmware-tanzu/nsx-operator/pkg/apis/vpc/v1alpha1"
	scheme "github.com/vmware-tanzu/nsx-operator/pkg/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// AddressBindingsGetter has a method to return a AddressBindingInterface.
// A group's client should implement this interface.
type AddressBindingsGetter interface {
	AddressBindings(namespace string) AddressBindingInterface
}

// AddressBindingInterface has methods to work with AddressBinding resources.
type AddressBindingInterface interface {
	Create(ctx context.Context, addressBinding *v1alpha1.AddressBinding, opts v1.CreateOptions) (*v1alpha1.AddressBinding, error)
	Update(ctx context.Context, addressBinding *v1alpha1.AddressBinding, opts v1.UpdateOptions) (*v1alpha1.AddressBinding, error)
	UpdateStatus(ctx context.Context, addressBinding *v1alpha1.AddressBinding, opts v1.UpdateOptions) (*v1alpha1.AddressBinding, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.AddressBinding, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.AddressBindingList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.AddressBinding, err error)
	AddressBindingExpansion
}

// addressBindings implements AddressBindingInterface
type addressBindings struct {
	client rest.Interface
	ns     string
}

// newAddressBindings returns a AddressBindings
func newAddressBindings(c *CrdV1alpha1Client, namespace string) *addressBindings {
	return &addressBindings{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the addressBinding, and returns the corresponding addressBinding object, and an error if there is any.
func (c *addressBindings) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.AddressBinding, err error) {
	result = &v1alpha1.AddressBinding{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("addressbindings").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of AddressBindings that match those selectors.
func (c *addressBindings) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.AddressBindingList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.AddressBindingList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("addressbindings").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested addressBindings.
func (c *addressBindings) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("addressbindings").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a addressBinding and creates it.  Returns the server's representation of the addressBinding, and an error, if there is any.
func (c *addressBindings) Create(ctx context.Context, addressBinding *v1alpha1.AddressBinding, opts v1.CreateOptions) (result *v1alpha1.AddressBinding, err error) {
	result = &v1alpha1.AddressBinding{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("addressbindings").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(addressBinding).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a addressBinding and updates it. Returns the server's representation of the addressBinding, and an error, if there is any.
func (c *addressBindings) Update(ctx context.Context, addressBinding *v1alpha1.AddressBinding, opts v1.UpdateOptions) (result *v1alpha1.AddressBinding, err error) {
	result = &v1alpha1.AddressBinding{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("addressbindings").
		Name(addressBinding.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(addressBinding).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *addressBindings) UpdateStatus(ctx context.Context, addressBinding *v1alpha1.AddressBinding, opts v1.UpdateOptions) (result *v1alpha1.AddressBinding, err error) {
	result = &v1alpha1.AddressBinding{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("addressbindings").
		Name(addressBinding.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(addressBinding).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the addressBinding and deletes it. Returns an error if one occurs.
func (c *addressBindings) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("addressbindings").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *addressBindings) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("addressbindings").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched addressBinding.
func (c *addressBindings) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.AddressBinding, err error) {
	result = &v1alpha1.AddressBinding{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("addressbindings").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
