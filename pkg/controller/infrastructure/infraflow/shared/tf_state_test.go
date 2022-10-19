// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shared_test

import (
	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure/infraflow/shared"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
)

var _ = Describe("TerraformState", func() {
	It("should unmarshall terraformer state", func() {
		var ()

		rawState := &terraformer.RawState{
			Data:     tfstate,
			Encoding: "none",
		}

		tf, err := shared.UnmarshalTerraformStateFromTerraformer(rawState)
		Expect(err).NotTo(HaveOccurred())

		Expect(tf.Outputs["vpc_id"]).To(Equal(shared.TFOutput{Type: "string", Value: "vpc-0123456"}))

		tables := tf.FindManagedResourcesByType("aws_route_table")
		Expect(len(tables)).To(Equal(2))

		Expect(tf.GetManagedResourceInstanceID("aws_route_table", "routetable_private_utility_z0")).
			To(Equal(pointer.String("rtb-77777")))

		Expect(tf.GetManagedResourceInstanceName("aws_iam_role", "nodes")).
			To(Equal(pointer.String("shoot--foo--bar-nodes")))

		Expect(tf.GetManagedResourceInstanceAttribute("aws_nat_gateway", "natgw_z0", "private_ip")).
			To(Equal(pointer.String("10.180.46.81")))

		Expect(tf.GetManagedResourceInstanceAttribute("aws_nat_gateway", "natgw_z0", "foobar")).
			To(BeNil())

		Expect(tf.GetManagedResourceInstances("aws_route_table")).
			To(Equal(map[string]string{
				"routetable_main":               "rtb-66666",
				"routetable_private_utility_z0": "rtb-77777",
			}))
	})
})

const tfstate = `{
  "version": 4,
  "terraform_version": "0.15.5",
  "serial": 83,
  "lineage": "674a5a9a-d0e5-eee1-ce57-d820c4313bf0",
  "outputs": {
    "vpc_id": {
      "value": "vpc-0123456",
      "type": "string"
    }
  },
  "resources": [
    {
      "mode": "managed",
      "type": "aws_iam_role",
      "name": "nodes",
      "provider": "provider[\"registry.terraform.io/hashicorp/aws\"]",
      "instances": [
        {
          "schema_version": 0,
          "attributes": {
            "id": "shoot--foo--bar-nodes-id",
            "max_session_duration": 3600,
            "name": "shoot--foo--bar-nodes"
          }
        }
      ]
    },
    {
      "mode": "managed",
      "type": "aws_nat_gateway",
      "name": "natgw_z0",
      "provider": "provider[\"registry.terraform.io/hashicorp/aws\"]",
      "instances": [
        {
          "schema_version": 0,
          "attributes": {
            "allocation_id": "eipalloc-07aaaaa",
            "connectivity_type": "public",
            "id": "nat-22222",
            "network_interface_id": "eni-33333",
            "private_ip": "10.180.46.81",
            "public_ip": "1.2.3.4",
            "subnet_id": "subnet-44444",
            "tags": {
              "Name": "shoot--foo--bar-natgw-z0",
              "kubernetes.io/cluster/shoot--foo--bar": "1"
            },
            "tags_all": {
              "Name": "shoot--foo--bar-natgw-z0",
              "kubernetes.io/cluster/shoot--foo--bar": "1"
            }
          },
          "sensitive_attributes": [],
          "private": "xxx",
          "dependencies": [
            "aws_eip.eip_natgw_z0",
            "aws_subnet.public_utility_z0",
            "aws_vpc.vpc"
          ]
        }
      ]
    },
    {
      "mode": "managed",
      "type": "aws_route_table",
      "name": "routetable_main",
      "provider": "provider[\"registry.terraform.io/hashicorp/aws\"]",
      "instances": [
        {
          "schema_version": 0,
          "attributes": {
            "id": "rtb-66666",
            "owner_id": "999999",
            "propagating_vgws": [],
            "route": [
              {
                "cidr_block": "0.0.0.0/0"
              }
            ],
            "tags": {
              "Name": "shoot--foo--bar",
              "kubernetes.io/cluster/shoot--foo--bar": "1"
            },
            "tags_all": {
              "Name": "shoot--foo--bar",
              "kubernetes.io/cluster/shoot--foo--bar": "1"
            },
            "timeouts": {
              "create": "5m",
              "delete": null,
              "update": null
            },
            "vpc_id": "vpc-0123456"
          },
          "sensitive_attributes": [],
          "private": "xxx",
          "dependencies": [
            "aws_vpc.vpc"
          ]
        }
      ]
    },
    {
      "mode": "managed",
      "type": "aws_route_table",
      "name": "routetable_private_utility_z0",
      "provider": "provider[\"registry.terraform.io/hashicorp/aws\"]",
      "instances": [
        {
          "schema_version": 0,
          "attributes": {
            "id": "rtb-77777",
            "owner_id": "999999",
            "route": [
              {
                "cidr_block": "0.0.0.0/0",
                "nat_gateway_id": "nat-22222"
              }
            ],
            "tags": {
              "Name": "shoot--foo--bar-private-eu-west-1a",
              "kubernetes.io/cluster/shoot--foo--bar": "1"
            },
            "tags_all": {
              "Name": "shoot--foo--bar-private-eu-west-1a",
              "kubernetes.io/cluster/shoot--foo--bar": "1"
            },
            "timeouts": {
              "create": "5m",
              "delete": null,
              "update": null
            },
            "vpc_id": "vpc-0123456"
          },
          "sensitive_attributes": [],
          "private": "xxx",
          "dependencies": [
            "aws_vpc.vpc"
          ]
        }
      ]
    },
    {
      "mode": "managed",
      "type": "aws_vpc",
      "name": "vpc",
      "provider": "provider[\"registry.terraform.io/hashicorp/aws\"]",
      "instances": [
        {
          "schema_version": 1,
          "attributes": {
            "arn": "arn:aws:ec2:eu-west-1:999999:vpc/vpc-0123456",
            "assign_generated_ipv6_cidr_block": false,
            "cidr_block": "10.180.0.0/16",
            "default_security_group_id": "sg-11111",
            "enable_classiclink": false,
            "enable_classiclink_dns_support": false,
            "enable_dns_hostnames": true,
            "enable_dns_support": true,
            "id": "vpc-0123456",
            "instance_tenancy": "default",
            "ipv6_association_id": "",
            "ipv6_cidr_block": "",
            "main_route_table_id": "rtb-77888",
            "owner_id": "999999",
            "tags": {
              "Name": "shoot--foo--bar",
              "kubernetes.io/cluster/shoot--foo--bar": "1"
            },
            "tags_all": {
              "Name": "shoot--foo--bar",
              "kubernetes.io/cluster/shoot--foo--bar": "1"
            }
          },
          "sensitive_attributes": [],
          "private": "eyJzY2hlbWFfdmVyc2lvbiI6IjEifQ=="
        }
      ]
    }
  ]
}
`
