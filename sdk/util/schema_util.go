// Copyright 2022, Pulumi Corporation.  All rights reserved.

package util

import (
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

func ResourceProperties(resource *schema.Resource) []*schema.Property {
	if resource == nil {
		return nil
	}
	// id and urn props are special and not part of the schema
	properties := []*schema.Property{
		{
			Name:    "id",
			Type:    schema.StringType,
			Plain:   false,
			Comment: "ID is a unique identifier assigned by a resource provider to a resource",
		},
		{
			Name:  "urn",
			Type:  schema.StringType,
			Plain: false,
			Comment: "URN is an automatically generated logical Uniform Resource Name, " +
				"used to stably identify resources. See " +
				"https://www.pulumi.com/docs/intro/concepts/resources/names/#urns",
		},
	}

	if resource.Properties != nil {
		properties = append(properties, resource.Properties...)
	}

	if resource.InputProperties != nil {
		properties = append(properties, resource.InputProperties...)
	}

	return properties
}
