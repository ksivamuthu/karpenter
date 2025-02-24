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

package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha4"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	v1alpha1 "github.com/awslabs/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/patrickmn/go-cache"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/logging"
)

const kubernetesVersionCacheKey = "kubernetesVersion"

type AMIProvider struct {
	cache     *cache.Cache
	ssm       ssmiface.SSMAPI
	clientSet *kubernetes.Clientset
}

func NewAMIProvider(ssm ssmiface.SSMAPI, clientSet *kubernetes.Clientset) *AMIProvider {
	return &AMIProvider{
		ssm:       ssm,
		clientSet: clientSet,
		cache:     cache.New(CacheTTL, CacheCleanupInterval),
	}
}

func (p *AMIProvider) getSSMParameter(ctx context.Context, constraints *v1alpha1.Constraints, instanceTypes []cloudprovider.InstanceType) (string, error) {
	version, err := p.kubeServerVersion(ctx)
	if err != nil {
		return "", fmt.Errorf("kube server version, %w", err)
	}
	var amiNameSuffix string
	if len(constraints.Architectures) > 0 {
		// select the first one if multiple supported
		if constraints.Architectures[0] == v1alpha4.ArchitectureArm64 {
			amiNameSuffix = "-arm64"
		}
	}
	if needsGPUAmi(instanceTypes) {
		if amiNameSuffix != "" {
			return "", fmt.Errorf("no amazon-linux-2 ami available for both nvidia/neuron gpus and arm64 cpus")
		}
		amiNameSuffix = "-gpu"
	}
	return fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2%s/recommended/image_id", version, amiNameSuffix), nil
}

func (p *AMIProvider) Get(ctx context.Context, constraints *v1alpha1.Constraints, instanceTypes []cloudprovider.InstanceType) (string, error) {
	name, err := p.getSSMParameter(ctx, constraints, instanceTypes)
	if err != nil {
		return "", err
	}
	if id, ok := p.cache.Get(name); ok {
		return id.(string), nil
	}
	output, err := p.ssm.GetParameterWithContext(ctx, &ssm.GetParameterInput{Name: aws.String(name)})
	if err != nil {
		return "", fmt.Errorf("getting ssm parameter, %w", err)
	}
	ami := aws.StringValue(output.Parameter.Value)
	p.cache.Set(name, ami, CacheTTL)
	logging.FromContext(ctx).Debugf("Discovered ami %s for query %s", ami, name)
	return ami, nil
}

func (p *AMIProvider) kubeServerVersion(ctx context.Context) (string, error) {
	if version, ok := p.cache.Get(kubernetesVersionCacheKey); ok {
		return version.(string), nil
	}
	serverVersion, err := p.clientSet.Discovery().ServerVersion()
	if err != nil {
		return "", err
	}
	version := fmt.Sprintf("%s.%s", serverVersion.Major, strings.TrimSuffix(serverVersion.Minor, "+"))
	p.cache.Set(kubernetesVersionCacheKey, version, CacheTTL)
	logging.FromContext(ctx).Debugf("Discovered kubernetes version %s", version)
	return version, nil
}
