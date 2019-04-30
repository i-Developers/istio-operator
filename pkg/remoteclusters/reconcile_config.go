/*
Copyright 2019 Banzai Cloud.

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

package remoteclusters

import (
	"context"

	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	istiov1beta1 "github.com/banzaicloud/istio-operator/pkg/apis/istio/v1beta1"
	"github.com/banzaicloud/istio-operator/pkg/util"
)

const ConfigName = "istio-config"

func (c *Cluster) reconcileConfig(remoteConfig *istiov1beta1.RemoteIstio, istio *istiov1beta1.Istio) error {
	c.log.Info("reconciling config")

	var istioConfig istiov1beta1.Istio
	err := c.ctrlRuntimeClient.Get(context.TODO(), types.NamespacedName{
		Name:      ConfigName,
		Namespace: remoteConfig.Namespace,
	}, &istioConfig)
	if err != nil && !k8sapierrors.IsNotFound(err) {
		return err
	}

	istioConfig.Spec = istio.Spec
	istioConfig.Spec.AutoInjectionNamespaces = remoteConfig.Spec.AutoInjectionNamespaces
	istioConfig.Spec.SidecarInjector.ReplicaCount = remoteConfig.Spec.SidecarInjector.ReplicaCount
	istioConfig.Spec.Proxy.Privileged = remoteConfig.Spec.Proxy.Privileged

	if util.PointerToBool(istioConfig.Spec.MeshExpansion) {
		istioConfig.Spec.Gateways.IngressConfig.Enabled = util.BoolPointer(true)
		istioConfig.Spec.SetNetworkName(remoteConfig.Name)
	}

	if k8sapierrors.IsNotFound(err) {
		istioConfig.Name = ConfigName
		istioConfig.Namespace = remoteConfig.Namespace

		err = c.ctrlRuntimeClient.Create(context.TODO(), &istioConfig)
		if err != nil {
			return err
		}
	} else {
		err = c.ctrlRuntimeClient.Update(context.TODO(), &istioConfig)
		if err != nil {
			return err
		}
	}

	crd := c.configcrd()
	istioConfig.TypeMeta.Kind = crd.Spec.Names.Kind
	istioConfig.TypeMeta.APIVersion = crd.Spec.Group + "/" + crd.Spec.Version
	c.istioConfig = &istioConfig

	return nil
}
