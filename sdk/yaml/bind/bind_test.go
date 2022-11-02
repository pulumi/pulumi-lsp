// Copyright 2022, Pulumi Corporation.  All rights reserved.

package bind

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	yaml "github.com/pulumi/pulumi-yaml/pkg/pulumiyaml"
	"github.com/pulumi/pulumi-yaml/pkg/pulumiyaml/ast"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.lsp.dev/protocol"

	"github.com/pulumi/pulumi-lsp/sdk/lsp"
)

const awsEksExample = `
name: aws-eks
runtime: yaml
description: An EKS cluster
variables:
  vpcId:
    fn::invoke:
      function: aws:ec2:getVpc
      arguments:
        default: true
      return: id
  subnetIds:
    fn::invoke:
      function: aws:ec2:getSubnetIds
      arguments:
        vpcId: ${vpcId}
      return: ids
resources:
  cluster:
    type: eks:Cluster
    properties:
      vpcId: ${vpcId}
      subnetIds: ${subnetIds}
      instanceType: "t2.medium"
      desiredCapacity: 2
      minSize: 1
      maxSize: 2
outputs:
  kubeconfig: ${cluster.kubeconfig}
`

func newDocument(name, body string) lsp.Document {
	return lsp.NewDocument(protocol.TextDocumentItem{
		URI:        protocol.DocumentURI("file://" + name),
		LanguageID: protocol.YamlLanguage,
		Text:       body,
	})
}

func withLineAs(doc lsp.Document, lineNumber int, line string) {
	existing, err := doc.Line(lineNumber)
	if err != nil {
		panic(err)
	}
	err = doc.AcceptChanges([]protocol.TextDocumentContentChangeEvent{{
		Range: protocol.Range{
			Start: protocol.Position{
				Line:      uint32(lineNumber),
				Character: 0,
			},
			End: protocol.Position{
				Line:      uint32(lineNumber),
				Character: uint32(len(existing)),
			},
		},
		RangeLength: uint32(len(line)),
		Text:        line,
	}})
	if err != nil {
		panic(err)
	}
}

func cleanParse(t *testing.T, doc lsp.Document) *ast.TemplateDecl {
	parsed, diags, err := yaml.LoadYAML(doc.URI().Filename(), strings.NewReader(doc.String()))
	require.NoError(t, err)
	assert.Len(t, diags, 0)
	return parsed
}

// Create a simple range on a single line of ASCI text.
func rangeOnLine(line, startByte, start, end int) *hcl.Range {
	return &hcl.Range{
		Start: hcl.Pos{
			Line:   line,
			Column: start,
			Byte:   startByte,
		},
		End: hcl.Pos{
			Line:   line,
			Column: end,
			Byte:   startByte + end - start,
		},
	}
}

func TestBind(t *testing.T) {
	doc := newDocument("invalid-property", awsEksExample)
	withLineAs(doc, 28, "  kubeconfig: ${cluster.kubeconfigg}")
	parsed := cleanParse(t, doc)
	decl, err := NewDecl(parsed)
	require.NoError(t, err)
	diags := decl.Diags()
	require.Len(t, diags, 0)
	decl.LoadSchema(rootPluginLoader)
	diags = decl.Diags()
	require.Len(t, diags, 1)
	assert.Equal(t, &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Property 'kubeconfigg' does not exist on eks:index:Cluster",
		Detail:   "Existing properties are: kubeconfig, core, minSize, nodeAmiId, roleMappings, subnetIds, urn, userMappings, version, awsProvider, clusterTags, id, maxSize, name, provider, proxy, tags, vpcId, eksCluster, fargate, gpu, nodePublicKey, nodeSubnetIds, nodeUserData, publicSubnetIds, serviceRole, instanceRole, instanceRoles, instanceType, vpcCniOptions, desiredCapacity, nodeGroupOptions, publicAccessCidrs, storageClasses, defaultNodeGroup, privateSubnetIds, createOidcProvider, nodeRootVolumeSize, nodeSecurityGroup, useDefaultVpcCni, instanceProfileName, nodeRootVolumeIops, nodeRootVolumeType, clusterSecurityGroup, creationRoleProvider, eksClusterIngressRule, encryptionConfigKeyArn, endpointPublicAccess, nodeSecurityGroupTags, skipDefaultNodeGroup, clusterSecurityGroupTags, enabledClusterLogTypes, endpointPrivateAccess, providerCredentialOpts, encryptRootBlockDevice, nodeRootVolumeEncrypted, nodeRootVolumeThroughput, kubernetesServiceIpAddressRange, nodeAssociatePublicIpAddress, nodeRootVolumeDeleteOnTermination",
		Subject:  rangeOnLine(29, 10, 25, 36),
	}, diags[0])
}

func TestBindProperty(t *testing.T) {
	doc := newDocument("property-binding", `
variables:
  binding: ${pulumi.foo}
`)
	parsed := cleanParse(t, doc)
	decl, err := NewDecl(parsed)
	require.NoError(t, err)
	uses := decl.variables["pulumi"].uses
	require.Len(t, uses, 1)
	use := uses[0]
	assert.Equal(t, &hcl.Range{
		Start: hcl.Pos{
			Line:   3,
			Column: 12,
		},
		End: hcl.Pos{
			Line:   3,
			Column: 25,
		},
	}, use.Range())
	assert.Equal(t, rangeOnLine(3, 9, 21, 24), use.access[0].rnge)
}

func newPluginLoader() schema.ReferenceLoader {
	schemaLoadPath := filepath.Join("..", "testdata")
	return schema.NewPluginLoader(utils.NewHost(schemaLoadPath))
}

var rootPluginLoader schema.ReferenceLoader = newPluginLoader()
