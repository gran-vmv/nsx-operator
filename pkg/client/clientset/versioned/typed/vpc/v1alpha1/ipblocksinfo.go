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

// IPBlocksInfosGetter has a method to return a IPBlocksInfoInterface.
// A group's client should implement this interface.
type IPBlocksInfosGetter interface {
	IPBlocksInfos() IPBlocksInfoInterface
}

// IPBlocksInfoInterface has methods to work with IPBlocksInfo resources.
type IPBlocksInfoInterface interface {
	Create(ctx context.Context, iPBlocksInfo *v1alpha1.IPBlocksInfo, opts v1.CreateOptions) (*v1alpha1.IPBlocksInfo, error)
	Update(ctx context.Context, iPBlocksInfo *v1alpha1.IPBlocksInfo, opts v1.UpdateOptions) (*v1alpha1.IPBlocksInfo, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.IPBlocksInfo, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.IPBlocksInfoList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.IPBlocksInfo, err error)
	IPBlocksInfoExpansion
}

// iPBlocksInfos implements IPBlocksInfoInterface
type iPBlocksInfos struct {
	client rest.Interface
}

// newIPBlocksInfos returns a IPBlocksInfos
func newIPBlocksInfos(c *CrdV1alpha1Client) *iPBlocksInfos {
	return &iPBlocksInfos{
		client: c.RESTClient(),
	}
}

// Get takes name of the iPBlocksInfo, and returns the corresponding iPBlocksInfo object, and an error if there is any.
func (c *iPBlocksInfos) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.IPBlocksInfo, err error) {
	result = &v1alpha1.IPBlocksInfo{}
	err = c.client.Get().
		Resource("ipblocksinfos").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of IPBlocksInfos that match those selectors.
func (c *iPBlocksInfos) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.IPBlocksInfoList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.IPBlocksInfoList{}
	err = c.client.Get().
		Resource("ipblocksinfos").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested iPBlocksInfos.
func (c *iPBlocksInfos) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("ipblocksinfos").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a iPBlocksInfo and creates it.  Returns the server's representation of the iPBlocksInfo, and an error, if there is any.
func (c *iPBlocksInfos) Create(ctx context.Context, iPBlocksInfo *v1alpha1.IPBlocksInfo, opts v1.CreateOptions) (result *v1alpha1.IPBlocksInfo, err error) {
	result = &v1alpha1.IPBlocksInfo{}
	err = c.client.Post().
		Resource("ipblocksinfos").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(iPBlocksInfo).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a iPBlocksInfo and updates it. Returns the server's representation of the iPBlocksInfo, and an error, if there is any.
func (c *iPBlocksInfos) Update(ctx context.Context, iPBlocksInfo *v1alpha1.IPBlocksInfo, opts v1.UpdateOptions) (result *v1alpha1.IPBlocksInfo, err error) {
	result = &v1alpha1.IPBlocksInfo{}
	err = c.client.Put().
		Resource("ipblocksinfos").
		Name(iPBlocksInfo.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(iPBlocksInfo).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the iPBlocksInfo and deletes it. Returns an error if one occurs.
func (c *iPBlocksInfos) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("ipblocksinfos").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *iPBlocksInfos) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("ipblocksinfos").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched iPBlocksInfo.
func (c *iPBlocksInfos) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.IPBlocksInfo, err error) {
	result = &v1alpha1.IPBlocksInfo{}
	err = c.client.Patch(pt).
		Resource("ipblocksinfos").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
