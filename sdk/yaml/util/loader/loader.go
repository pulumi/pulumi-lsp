// Copyright 2022, Pulumi Corporation.  All rights reserved.

package loader

import (
	"context"
	"reflect"
	"sync"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

type ReferenceLoader interface {
	schema.ReferenceLoader

	Loaded() []schema.PackageDescriptor
}

func New(host plugin.Host) ReferenceLoader {
	return &refLoader{inner: schema.NewPluginLoader(host)}
}

type refLoader struct {
	inner schema.ReferenceLoader

	m      sync.Mutex
	loaded []schema.PackageDescriptor
}

func (r *refLoader) Loaded() []schema.PackageDescriptor {
	r.m.Lock()
	defer r.m.Unlock()

	out := make([]schema.PackageDescriptor, len(r.loaded))
	copy(out, r.loaded)
	return out
}

// deprecated: use LoadPackageV2
func (r *refLoader) LoadPackage(pkg string, version *semver.Version) (*schema.Package, error) {
	p, err := r.inner.LoadPackage(pkg, version)
	if err != nil {
		return p, err
	}
	r.push(&schema.PackageDescriptor{Name: pkg, Version: version})
	return p, err
}

func (r *refLoader) LoadPackageV2(ctx context.Context, descriptor *schema.PackageDescriptor) (*schema.Package, error) {
	p, err := r.inner.LoadPackageV2(ctx, descriptor)
	if err != nil {
		return p, err
	}
	r.push(descriptor)
	return p, err
}

// deprecated: use LoadPackageReferenceV2
func (r *refLoader) LoadPackageReference(pkg string, version *semver.Version) (schema.PackageReference, error) {
	p, err := r.inner.LoadPackageReference(pkg, version)
	if err != nil {
		return p, err
	}
	r.push(&schema.PackageDescriptor{Name: pkg, Version: version})
	return p, err
}

func (r *refLoader) LoadPackageReferenceV2(ctx context.Context, descriptor *schema.PackageDescriptor) (schema.PackageReference, error) {
	p, err := r.inner.LoadPackageReferenceV2(ctx, descriptor)
	if err != nil {
		return p, err
	}
	r.push(descriptor)
	return p, err
}

func (r *refLoader) push(d *schema.PackageDescriptor) {
	if d == nil {
		return
	}
	r.m.Lock()
	defer r.m.Unlock()

	for _, o := range r.loaded {
		if reflect.DeepEqual(*d, o) {
			return
		}
	}

	r.loaded = append(r.loaded, *d)
}
