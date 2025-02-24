/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cloudprovider

import (
	"context"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha4"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"
)

// CloudProvider interface is implemented by cloud providers to support provisioning.
type CloudProvider interface {
	// Create a set of nodes for each of the given constraints. This API uses a
	// callback pattern to enable cloudproviders to batch capacity creation
	// requests. The callback must be called with a theoretical node object that
	// is fulfilled by the cloud providers capacity creation request. This API
	// is called in parallel and then waits for all channels to return nil or error.
	Create(context.Context, *v1alpha4.Constraints, []InstanceType, func(*v1.Node) error) chan error
	// Delete node in cloudprovider
	Delete(context.Context, *v1.Node) error
	// GetInstanceTypes returns the instance types supported by the cloud
	// provider limited by the provided constraints and daemons.
	GetInstanceTypes(context.Context) ([]InstanceType, error)
	// GetZones returns the zones supported by the cloud provider and the specified provisioner
	GetZones(context.Context, *v1alpha4.Constraints) ([]string, error)
	// Validate is a hook for additional validation logic. This method is not
	// only called during Provisioner CRD validation, it is also used at
	// provisioning time to ensure that pods are provisionable.
	Validate(context.Context, *v1alpha4.Constraints) *apis.FieldError
	// Default is a hook for additional defaulting logic specific.
	Default(context.Context, *v1alpha4.Constraints)
}

// Options are injected into cloud providers' factories
type Options struct {
	ClientSet *kubernetes.Clientset
}

// InstanceType describes the properties of a potential node
type InstanceType interface {
	Name() string
	Zones() []string
	Architectures() []string
	OperatingSystems() []string
	CPU() *resource.Quantity
	Memory() *resource.Quantity
	Pods() *resource.Quantity
	NvidiaGPUs() *resource.Quantity
	AMDGPUs() *resource.Quantity
	AWSNeurons() *resource.Quantity
	Overhead() v1.ResourceList
}
