// Copyright 2022, Pulumi Corporation.  All rights reserved.

package util

import (
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

func ResourceProperties(resource *schema.Resource) []*schema.Property {
	properties := []*schema.Property{
		// ID property is special and not part of the schema.
		// TODO should this also do URN property?
		{
			Name:  "id",
			Type:  schema.StringType,
			Plain: false,
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
