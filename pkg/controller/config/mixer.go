package config

import (
	"fmt"

	istiov1beta1 "github.com/banzaicloud/istio-operator/pkg/apis/operator/v1beta1"
	"github.com/banzaicloud/istio-operator/pkg/k8sutil"
	"github.com/banzaicloud/istio-operator/pkg/util"
	"github.com/go-logr/logr"
	"github.com/goph/emperror"
	"istio.io/api/networking/v1alpha3"
	configv1alpha2 "istio.io/api/pkg/kube/apis/config/v1alpha2"
	networkingv1alpha3 "istio.io/api/pkg/kube/apis/networking/v1alpha3"
	"istio.io/api/policy/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalev2beta1 "k8s.io/api/autoscaling/v2beta1"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ReconcileConfig) ReconcileMixer(log logr.Logger, istio *istiov1beta1.Config) error {

	mixerResources := make(map[string]runtime.Object)

	mixerSa := &apiv1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "istio-mixer-service-account",
			Namespace: istio.Namespace,
			Labels: map[string]string{
				"app": "mixer",
			},
		},
	}
	controllerutil.SetControllerReference(istio, mixerSa, r.scheme)
	mixerResources[mixerSa.Name] = mixerSa

	mixerCr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "istio-mixer-cluster-role",
			Labels: map[string]string{
				"app": "mixer",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"config.istio.io"},
				Resources: []string{"*"},
				Verbs:     []string{"create", "get", "list", "watch", "patch"},
			},
			{
				APIGroups: []string{"rbac.istio.io"},
				Resources: []string{"*"},
				Verbs:     []string{"get", "watch", "list"},
			},
			{
				APIGroups: []string{"apiextensions.k8s.io"},
				Resources: []string{"customresourcedefinitions"},
				Verbs:     []string{"get", "watch", "list"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps", "endpoints", "pods", "services", "namespaces", "secrets", "replicationcontrollers"},
				Verbs:     []string{"get", "watch", "list"},
			},
			{
				APIGroups: []string{"extensions"},
				Resources: []string{"replicasets"},
				Verbs:     []string{"get", "watch", "list"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"replicasets"},
				Verbs:     []string{"get", "watch", "list"},
			},
		},
	}
	controllerutil.SetControllerReference(istio, mixerCr, r.scheme)
	mixerResources[mixerCr.Name] = mixerCr

	mixerCrb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "istio-mixer-cluster-role-binding",
			Labels: map[string]string{
				"app": "mixer",
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			APIGroup: "rbac.authorization.k8s.io",
			Name:     mixerCr.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      mixerSa.Name,
				Namespace: istio.Namespace,
			},
		},
	}
	controllerutil.SetControllerReference(istio, mixerCrb, r.scheme)
	mixerResources[mixerCrb.Name] = mixerCrb

	policyDeploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "istio-policy-deployment",
			Namespace: istio.Namespace,
			Labels: map[string]string{
				"istio": "mixer",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: util.IntPointer(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"istio":            "mixer",
					"app":              "policy",
					"istio-mixer-type": "policy",
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"istio":            "mixer",
						"app":              "policy",
						"istio-mixer-type": "policy",
					},
					Annotations: defaultDeployAnnotations(),
				},
				Spec: apiv1.PodSpec{
					ServiceAccountName: mixerSa.Name,
					Volumes:            mixerVolumes(mixerSa.Name),
					Affinity:           &apiv1.Affinity{},
					Containers: []apiv1.Container{
						mixerContainer(true, istio.Namespace),
						istioProxyContainer("policy"),
					},
				},
			},
		},
	}
	controllerutil.SetControllerReference(istio, policyDeploy, r.scheme)
	mixerResources[policyDeploy.Name] = policyDeploy

	telemetryDeploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "istio-telemetry-deployment",
			Namespace: istio.Namespace,
			Labels: map[string]string{
				"istio": "mixer",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: util.IntPointer(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"istio":            "mixer",
					"app":              "telemetry",
					"istio-mixer-type": "telemetry",
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"istio":            "mixer",
						"app":              "telemetry",
						"istio-mixer-type": "telemetry",
					},
					Annotations: defaultDeployAnnotations(),
				},
				Spec: apiv1.PodSpec{
					ServiceAccountName: mixerSa.Name,
					Volumes:            mixerVolumes(mixerSa.Name),
					Affinity:           &apiv1.Affinity{},
					Containers: []apiv1.Container{
						mixerContainer(false, istio.Namespace),
						istioProxyContainer("telemetry"),
					},
				},
			},
		},
	}
	controllerutil.SetControllerReference(istio, telemetryDeploy, r.scheme)
	mixerResources[telemetryDeploy.Name] = telemetryDeploy

	for _, mixer := range []string{"policy", "telemetry"} {
		mixerSvc := mixerService(mixer, istio.Namespace)
		controllerutil.SetControllerReference(istio, mixerSvc, r.scheme)
		mixerResources[mixerSvc.Name] = mixerSvc

		mixerAs := mixerAutoscaler(mixer, istio.Namespace)
		controllerutil.SetControllerReference(istio, mixerAs, r.scheme)
		mixerResources[mixerAs.Name] = mixerAs
	}

	// not sure if it's needed or not
	statsdPromBridgeCm := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "istio-statsd-prom-bridge",
			Namespace: istio.Namespace,
			Labels: map[string]string{
				"app":   "istio-statsd-prom-bridge",
				"istio": "mixer",
			},
		},
		Data: map[string]string{
			"mapping.conf": "",
		},
	}
	controllerutil.SetControllerReference(istio, statsdPromBridgeCm, r.scheme)
	mixerResources[statsdPromBridgeCm.Name] = statsdPromBridgeCm

	crs := r.mixerCustomResources(istio)
	for name, cr := range crs {
		mixerResources[name] = cr
	}

	for name, res := range mixerResources {
		err := k8sutil.ReconcileResource(log, r.Client, istio.Namespace, name, res)
		if err != nil {
			return emperror.WrapWith(err, "failed to reconcile resource", "resource", res.GetObjectKind().GroupVersionKind().Kind, "name", name)
		}
	}

	dcrs := r.mixerDynamicCustomResources(istio)
	for _, res := range dcrs {
		err := res.Reconcile(log, r.dynamic)
		if err != nil {
			return emperror.WrapWith(err, "failed to reconcile dynamic resource", "resource", res.Gvr.Resource, "name", res.Name)
		}
	}

	return nil
}

func (r *ReconcileConfig) mixerCustomResources(istio *istiov1beta1.Config) map[string]runtime.Object {
	crs := make(map[string]runtime.Object)

	istioproxy := &configv1alpha2.AttributeManifest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "istioproxy",
			Namespace: istio.Namespace,
		},
		Spec: v1beta1.AttributeManifest{
			Attributes: map[string]*v1beta1.AttributeManifest_AttributeInfo{
				"origin.ip":                        {ValueType: v1beta1.IP_ADDRESS},
				"origin.uid":                       {ValueType: v1beta1.STRING},
				"origin.user":                      {ValueType: v1beta1.STRING},
				"request.headers":                  {ValueType: v1beta1.STRING_MAP},
				"request.id":                       {ValueType: v1beta1.STRING},
				"request.host":                     {ValueType: v1beta1.STRING},
				"request.method":                   {ValueType: v1beta1.STRING},
				"request.path":                     {ValueType: v1beta1.STRING},
				"request.reason":                   {ValueType: v1beta1.STRING},
				"request.referer":                  {ValueType: v1beta1.STRING},
				"request.scheme":                   {ValueType: v1beta1.STRING},
				"request.total_size":               {ValueType: v1beta1.INT64},
				"request.size":                     {ValueType: v1beta1.INT64},
				"request.time":                     {ValueType: v1beta1.TIMESTAMP},
				"request.useragent":                {ValueType: v1beta1.STRING},
				"response.code":                    {ValueType: v1beta1.INT64},
				"response.duration":                {ValueType: v1beta1.DURATION},
				"response.headers":                 {ValueType: v1beta1.STRING_MAP},
				"response.total_size":              {ValueType: v1beta1.INT64},
				"response.size":                    {ValueType: v1beta1.INT64},
				"response.time":                    {ValueType: v1beta1.TIMESTAMP},
				"source.uid":                       {ValueType: v1beta1.STRING},
				"source.user":                      {ValueType: v1beta1.STRING},
				"source.principal":                 {ValueType: v1beta1.STRING},
				"destination.uid":                  {ValueType: v1beta1.STRING},
				"destination.port":                 {ValueType: v1beta1.INT64},
				"destination.principal":            {ValueType: v1beta1.STRING},
				"connection.event":                 {ValueType: v1beta1.STRING},
				"connection.id":                    {ValueType: v1beta1.STRING},
				"connection.received.bytes":        {ValueType: v1beta1.INT64},
				"connection.received.bytes_total":  {ValueType: v1beta1.INT64},
				"connection.sent.bytes":            {ValueType: v1beta1.INT64},
				"connection.sent.bytes_total":      {ValueType: v1beta1.INT64},
				"connection.v1beta1.DURATION":      {ValueType: v1beta1.DURATION},
				"connection.mtls":                  {ValueType: v1beta1.BOOL},
				"connection.requested_server_name": {ValueType: v1beta1.STRING},
				"context.protocol":                 {ValueType: v1beta1.STRING},
				"context.v1beta1.TIMESTAMP":        {ValueType: v1beta1.TIMESTAMP},
				"context.time":                     {ValueType: v1beta1.TIMESTAMP},
				"context.reporter.local":           {ValueType: v1beta1.BOOL},
				"context.reporter.kind":            {ValueType: v1beta1.STRING},
				"context.reporter.uid":             {ValueType: v1beta1.STRING},
				"api.service":                      {ValueType: v1beta1.STRING},
				"api.version":                      {ValueType: v1beta1.STRING},
				"api.operation":                    {ValueType: v1beta1.STRING},
				"api.protocol":                     {ValueType: v1beta1.STRING},
				"request.auth.principal":           {ValueType: v1beta1.STRING},
				"request.auth.audiences":           {ValueType: v1beta1.STRING},
				"request.auth.presenter":           {ValueType: v1beta1.STRING},
				"request.auth.claims":              {ValueType: v1beta1.STRING_MAP},
				"request.auth.raw_claims":          {ValueType: v1beta1.STRING},
				"request.api_key":                  {ValueType: v1beta1.STRING},
			},
		},
	}
	controllerutil.SetControllerReference(istio, istioproxy, r.scheme)
	crs[istioproxy.Name] = istioproxy

	kubernetes := &configv1alpha2.AttributeManifest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubernetes",
			Namespace: istio.Namespace,
		},
		Spec: v1beta1.AttributeManifest{
			Attributes: map[string]*v1beta1.AttributeManifest_AttributeInfo{
				"source.ip":                      {ValueType: v1beta1.IP_ADDRESS},
				"source.labels":                  {ValueType: v1beta1.STRING_MAP},
				"source.metadata":                {ValueType: v1beta1.STRING_MAP},
				"source.name":                    {ValueType: v1beta1.STRING},
				"source.namespace":               {ValueType: v1beta1.STRING},
				"source.owner":                   {ValueType: v1beta1.STRING},
				"source.service":                 {ValueType: v1beta1.STRING},
				"source.serviceAccount":          {ValueType: v1beta1.STRING},
				"source.services":                {ValueType: v1beta1.STRING},
				"source.workload.uid":            {ValueType: v1beta1.STRING},
				"source.workload.name":           {ValueType: v1beta1.STRING},
				"source.workload.namespace":      {ValueType: v1beta1.STRING},
				"destination.ip":                 {ValueType: v1beta1.IP_ADDRESS},
				"destination.labels":             {ValueType: v1beta1.STRING_MAP},
				"destination.metadata":           {ValueType: v1beta1.STRING_MAP},
				"destination.owner":              {ValueType: v1beta1.STRING},
				"destination.name":               {ValueType: v1beta1.STRING},
				"destination.container.name":     {ValueType: v1beta1.STRING},
				"destination.namespace":          {ValueType: v1beta1.STRING},
				"destination.service":            {ValueType: v1beta1.STRING},
				"destination.service.uid":        {ValueType: v1beta1.STRING},
				"destination.service.name":       {ValueType: v1beta1.STRING},
				"destination.service.namespace":  {ValueType: v1beta1.STRING},
				"destination.service.host":       {ValueType: v1beta1.STRING},
				"destination.serviceAccount":     {ValueType: v1beta1.STRING},
				"destination.workload.uid":       {ValueType: v1beta1.STRING},
				"destination.workload.name":      {ValueType: v1beta1.STRING},
				"destination.workload.namespace": {ValueType: v1beta1.STRING},
			},
		},
	}
	controllerutil.SetControllerReference(istio, kubernetes, r.scheme)
	crs[kubernetes.Name] = kubernetes

	istioPolicyDr := destinationRule(istio.Namespace, "istio-policy")
	controllerutil.SetControllerReference(istio, istioPolicyDr, r.scheme)
	crs[istioPolicyDr.Name] = istioPolicyDr

	istioTelemetryDr := destinationRule(istio.Namespace, "istio-telemetry")
	controllerutil.SetControllerReference(istio, istioTelemetryDr, r.scheme)
	crs[istioTelemetryDr.Name] = istioTelemetryDr
	return crs
}

func destinationRule(ns, name string) *networkingv1alpha3.DestinationRule {
	return &networkingv1alpha3.DestinationRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: v1alpha3.DestinationRule{
			Host: fmt.Sprintf("%s.%s.svc.cluster.local", name, ns),
			TrafficPolicy: &v1alpha3.TrafficPolicy{
				ConnectionPool: &v1alpha3.ConnectionPoolSettings{
					Http: &v1alpha3.ConnectionPoolSettings_HTTPSettings{
						Http2MaxRequests:         10000,
						MaxRequestsPerConnection: 10000,
					},
				},
			},
		},
	}
}

func (r *ReconcileConfig) mixerDynamicCustomResources(istio *istiov1beta1.Config) []k8sutil.DynamicResource {
	return []k8sutil.DynamicResource{
		{
			Gvr: schema.GroupVersionResource{
				Group:    "config.istio.io",
				Version:  "v1alpha2",
				Resource: "stdios",
			},
			Kind:      "stdio",
			Name:      "handler",
			Namespace: istio.Namespace,
			Spec: map[string]interface{}{
				"outputAsJson": true,
			},
			Owner: istio,
		},
		{
			Gvr: schema.GroupVersionResource{
				Group:    "config.istio.io",
				Version:  "v1alpha2",
				Resource: "logentries",
			},
			Kind:      "logentry",
			Name:      "accesslog",
			Namespace: istio.Namespace,
			Spec: map[string]interface{}{
				"severity":  `"Info"`,
				"timestamp": "request.time",
				"variables": map[string]interface{}{
					"sourceIp":                   `source.ip | ip("0.0.0.0")`,
					"sourceApp":                  `source.labels["app"] | ""`,
					"sourcePrincipal":            `source.principal | ""`,
					"sourceName":                 `source.name | ""`,
					"sourceWorkload":             `source.workload.name | ""`,
					"sourceNamespace":            `source.namespace | ""`,
					"sourceOwner":                `source.owner | ""`,
					"destinationApp":             `destination.labels["app"] | ""`,
					"destinationIp":              `destination.ip | ip("0.0.0.0")`,
					"destinationServiceHost":     `destination.service.host | ""`,
					"destinationWorkload":        `destination.workload.name | ""`,
					"destinationName":            `destination.name | ""`,
					"destinationNamespace":       `destination.namespace | ""`,
					"destinationOwner":           `destination.owner | ""`,
					"destinationPrincipal":       `destination.principal | ""`,
					"apiClaims":                  `request.auth.raw_claims | ""`,
					"apiKey":                     `request.api_key | request.headers["x-api-key"] | ""`,
					"protocol":                   `request.scheme | context.protocol | "http"`,
					"method":                     `request.method | ""`,
					"url":                        `request.path | ""`,
					"responseCode":               `response.code | 0`,
					"responseSize":               `response.size | 0`,
					"requestSize":                `request.size | 0`,
					"requestId":                  `request.headers["x-request-id"] | ""`,
					"clientTraceId":              `request.headers["x-client-trace-id"] | ""`,
					"latency":                    `response.duration | "0ms"`,
					"connection_security_policy": `conditional((context.reporter.kind | "inbound") == "outbound", "unknown", conditional(connection.mtls | false, "mutual_tls", "none"))`,
					"requestedServerName":        `connection.requested_server_name | ""`,
					"userAgent":                  `request.useragent | ""`,
					"responseTimestamp":          `response.time`,
					"receivedBytes":              `request.total_size | 0`,
					"sentBytes":                  `response.total_size | 0`,
					"referer":                    `request.referer | ""`,
					"httpAuthority":              `request.headers[":authority"] | request.host | ""`,
					"xForwardedFor":              `request.headers["x-forwarded-for"] | "0.0.0.0"`,
					"reporter":                   `conditional((context.reporter.kind | "inbound") == "outbound", "source", "destination")`,
				},
				"monitored_resource_type": `"global"`,
			},
			Owner: istio,
		},
		{
			Gvr: schema.GroupVersionResource{
				Group:    "config.istio.io",
				Version:  "v1alpha2",
				Resource: "logentries",
			},
			Kind:      "logentry",
			Name:      "tcpaccesslog",
			Namespace: istio.Namespace,
			Spec: map[string]interface{}{
				"severity":  `"Info"`,
				"timestamp": `context.time | timestamp("2017-01-01T00:00:00Z")`,
				"variables": map[string]interface{}{
					"connectionEvent":            `connection.event | ""`,
					"sourceIp":                   `source.ip | ip("0.0.0.0")`,
					"sourceApp":                  `source.labels["app"] | ""`,
					"sourcePrincipal":            `source.principal | ""`,
					"sourceName":                 `source.name | ""`,
					"sourceWorkload":             `source.workload.name | ""`,
					"sourceNamespace":            `source.namespace | ""`,
					"sourceOwner":                `source.owner | ""`,
					"destinationApp":             `destination.labels["app"] | ""`,
					"destinationIp":              `destination.ip | ip("0.0.0.0")`,
					"destinationServiceHost":     `destination.service.host | ""`,
					"destinationWorkload":        `destination.workload.name | ""`,
					"destinationName":            `destination.name | ""`,
					"destinationNamespace":       `destination.namespace | ""`,
					"destinationOwner":           `destination.owner | ""`,
					"destinationPrincipal":       `destination.principal | ""`,
					"protocol":                   `context.protocol | "tcp"`,
					"connectionDuration":         `connection.duration | "0ms"`,
					"connection_security_policy": `conditional((context.reporter.kind | "inbound") == "outbound", "unknown", conditional(connection.mtls | false, "mutual_tls", "none"))`,
					"requestedServerName":        `connection.requested_server_name | ""`,
					"receivedBytes":              `connection.received.bytes | 0`,
					"sentBytes":                  `connection.sent.bytes | 0`,
					"totalReceivedBytes":         `connection.received.bytes_total | 0`,
					"totalSentBytes":             `connection.sent.bytes_total | 0`,
					"reporter":                   `conditional((context.reporter.kind | "inbound") == "outbound", "source", "destination")`,
				},
				"monitored_resource_type": `"global"`,
			},
			Owner: istio,
		},
		rule("stdio", `context.protocol == "http" || context.protocol == "grpc"`, "handler.stdio", []string{"accesslog.logentry"}, istio),
		rule("stdiotcp", `context.protocol == "tcp"`, "handler.stdio", []string{"tcpaccesslog.logentry"}, istio),
		metric("requestcount", `"1"`, metricDimensions(), istio),
		metric("requestduration", `response.duration | "0ms"`, metricDimensions(), istio),
		metric("requestsize", `request.size | 0`, metricDimensions(), istio),
		metric("responsesize", `response.size | 0`, metricDimensions(), istio),
		metric("tcpbytesent", `connection.sent.bytes | 0`, tcpMetricDimensions(), istio),
		metric("tcpbytereceived", `connection.received.bytes | 0`, tcpMetricDimensions(), istio),
		{
			Gvr: schema.GroupVersionResource{
				Group:    "config.istio.io",
				Version:  "v1alpha2",
				Resource: "prometheuses",
			},
			Kind:      "prometheus",
			Name:      "handler",
			Namespace: istio.Namespace,
			Spec: map[string]interface{}{
				"metrics": []map[string]interface{}{
					{
						"name":          "requests_total",
						"instance_name": "requestcount.metric." + istio.Namespace,
						"kind":          "COUNTER",
						"label_names":   metricLabels(),
					},
					{
						"name":          "request_duration_seconds",
						"instance_name": "requestduration.metric." + istio.Namespace,
						"kind":          "DISTRIBUTION",
						"label_names":   metricLabels(),
						"buckets": map[string]interface{}{
							"explicit_buckets": map[string]interface{}{
								"bounds": "[0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10]",
							},
						},
					},
					{
						"name":          "request_bytes",
						"instance_name": "requestsize.metric." + istio.Namespace,
						"kind":          "DISTRIBUTION",
						"label_names":   metricLabels(),
						"buckets": map[string]interface{}{
							"exponentialBuckets": map[string]interface{}{
								"numFiniteBuckets": 8,
								"scale":            1,
								"growthFactor":     10,
							},
						},
					},
					{
						"name":          "response_bytes",
						"instance_name": "responsesize.metric." + istio.Namespace,
						"kind":          "DISTRIBUTION",
						"label_names":   metricLabels(),
						"buckets": map[string]interface{}{
							"exponentialBuckets": map[string]interface{}{
								"numFiniteBuckets": 8,
								"scale":            1,
								"growthFactor":     10,
							},
						},
					},
					{
						"name":          "tcp_sent_bytes_total",
						"instance_name": "tcpbytesent.metric." + istio.Namespace,
						"kind":          "COUNTER",
						"label_names":   tcpMetricLabels(),
					},
					{
						"name":          "tcp_received_bytes_total",
						"instance_name": "tcpbytereceived.metric." + istio.Namespace,
						"kind":          "COUNTER",
						"label_names":   tcpMetricLabels(),
					},
				},
			},
			Owner: istio,
		},
		rule("promhttp", `context.protocol == "http" || context.protocol == "grpc"`, "handler.prometheus",
			[]string{"requestcount.metric", "requestduration.metric", "requestsize.metric", "responsesize.metric"}, istio),
		rule("promtcp", `context.protocol == "tcp"`, "handler.prometheus",
			[]string{"tcpbytesent.metric", "tcpbytereceived.metric"}, istio),
		{
			Gvr: schema.GroupVersionResource{
				Group:    "config.istio.io",
				Version:  "v1alpha2",
				Resource: "kubernetesenvs",
			},
			Kind:      "kubernetesenv",
			Name:      "handler",
			Namespace: istio.Namespace,
			Spec:      map[string]interface{}{},
			Owner:     istio,
		},
		rule("kubeattrgenrulerule", "", "handler.kubernetesenv", []string{"attributes.kubernetes"}, istio),
		rule("tcpkubeattrgenrulerule", `context.protocol == "tcp"`, "handler.kubernetesenv", []string{"attributes.kubernetes"}, istio),
		{
			Gvr: schema.GroupVersionResource{
				Group:    "config.istio.io",
				Version:  "v1alpha2",
				Resource: "kuberneteses",
			},
			Kind:      "kubernetes",
			Name:      "attributes",
			Namespace: istio.Namespace,
			Spec: map[string]interface{}{
				"source_uid":       `source.uid | ""`,
				"source_ip":        `source.ip | ip("0.0.0.0")`,
				"destination_uid":  `destination.uid | ""`,
				"destination_port": `destination.port | 0`,
				"attribute_bindings": map[string]interface{}{
					"source.ip":                  `$out.source_pod_ip | ip("0.0.0.0")`,
					"source.uid":                 `$out.source_pod_uid | "unknown"`,
					"source.labels":              `$out.source_labels | emptyStringMap()`,
					"source.name":                `$out.source_pod_name | "unknown"`,
					"source.namespace":           `$out.source_namespace | "default"`,
					"source.owner":               `$out.source_owner | "unknown"`,
					"source.serviceAccount":      `$out.source_service_account_name | "unknown"`,
					"source.workload.uid":        `$out.source_workload_uid | "unknown"`,
					"source.workload.name":       `$out.source_workload_name | "unknown"`,
					"source.workload.namespace":  `$out.source_workload_namespace | "unknown"`,
					"destination.ip":             `$out.destination_pod_ip | ip("0.0.0.0")`,
					"destination.uid":            `$out.destination_pod_uid | "unknown"`,
					"destination.labels":         `$out.destination_labels | emptyStringMap()`,
					"destination.name":           `$out.destination_pod_name | "unknown"`,
					"destination.container.name": `$out.destination_container_name | "unknown"`,
					"destination.namespace":      `$out.destination_namespace | "default"`,
					"destination.owner":          `$out.destination_owner | "unknown"`,
					"destination.serviceAccount": `$out.destination_service_account_name | "unknown"`,
					"destination.workload.uid":   `$out.destination_workload_uid | "unknown"`,
					"destination.workload.name":  `$out.destination_workload_name | "unknown"`,
					"destination.workload.namespace": `$out.destination_workload_namespace | "unknown""	`,
				},
			},
			Owner: istio,
		},
		{
			Gvr: schema.GroupVersionResource{
				Group:    "config.istio.io",
				Version:  "v1alpha2",
				Resource: "MeshPolicies",
			},
			Kind: "MeshPolicy",
			Name: "default",
			Labels: map[string]string{
				"app": "istio-security",
			},
			Namespace: istio.Namespace,
			Spec: map[string]interface{}{
				"peers": []map[string]interface{}{
					{
						"mtls": map[string]interface{}{
							"mode": "PERMISSIVE",
						},
					},
				},
			},
			Owner: istio,
		},
	}
}

func iSlice(s ...string) []interface{} {
	ret := make([]interface{}, len(s))
	for i := 0; i < len(s); i++ {
		ret[i] = s[i]
	}
	return ret
}

func metricLabels() []interface{} {
	ml := tcpMetricLabels()
	ml = append(ml, "request_protocol")
	ml = append(ml, "response_code")
	return ml
}

func tcpMetricLabels() []interface{} {
	return iSlice([]string{
		"reporter",
		"source_app",
		"source_principal",
		"source_workload",
		"source_workload_namespace",
		"source_version",
		"destination_app",
		"destination_principal",
		"destination_workload",
		"destination_workload_namespace",
		"destination_version",
		"destination_service",
		"destination_service_name",
		"destination_service_namespace",
		"connection_security_policy",
	}...)
}

func rule(name, match, handler string, instances []string, owner *istiov1beta1.Config) k8sutil.DynamicResource {
	d := k8sutil.DynamicResource{
		Gvr: schema.GroupVersionResource{
			Group:    "config.istio.io",
			Version:  "v1alpha2",
			Resource: "rules",
		},
		Kind:      "rule",
		Name:      name,
		Namespace: owner.Namespace,
		Spec: map[string]interface{}{
			"actions": []interface{}{
				map[string]interface{}{
					"handler":   handler,
					"instances": iSlice(instances...),
				},
			},
		},
		Owner: owner,
	}
	if len(match) > 0 {
		d.Spec["match"] = match
	}
	return d
}

func metric(name, value string, dimensions map[string]interface{}, owner *istiov1beta1.Config) k8sutil.DynamicResource {
	return k8sutil.DynamicResource{
		Gvr: schema.GroupVersionResource{
			Group:    "config.istio.io",
			Version:  "v1alpha2",
			Resource: "metrics",
		},
		Kind:      "metric",
		Name:      name,
		Namespace: owner.Namespace,
		Spec: map[string]interface{}{
			"value":                   value,
			"dimensions":              dimensions,
			"monitored_resource_type": `"UNSPECIFIED"`,
		},
		Owner: owner,
	}
}

func metricDimensions() map[string]interface{} {
	md := tcpMetricDimensions()
	md["request_protocol"] = `api.protocol | context.protocol | "unknown"`
	md["response_code"] = `response.code | 200`
	return md
}

func tcpMetricDimensions() map[string]interface{} {
	return map[string]interface{}{
		"reporter":                       ` conditional((context.reporter.kind | "inbound") == "outbound", "source", "destination")`,
		"source_workload":                ` source.workload.name | "unknown"`,
		"source_workload_namespace":      ` source.workload.namespace | "unknown"`,
		"source_principal":               ` source.principal | "unknown"`,
		"source_app":                     ` source.labels["app"] | "unknown"`,
		"source_version":                 ` source.labels["version"] | "unknown"`,
		"destination_workload":           ` destination.workload.name | "unknown"`,
		"destination_workload_namespace": ` destination.workload.namespace | "unknown"`,
		"destination_principal":          ` destination.principal | "unknown"`,
		"destination_app":                ` destination.labels["app"] | "unknown"`,
		"destination_version":            ` destination.labels["version"] | "unknown"`,
		"destination_service":            ` destination.service.host | "unknown"`,
		"destination_service_name":       ` destination.service.name | "unknown"`,
		"destination_service_namespace":  ` destination.service.namespace | "unknown"`,
		"connection_security_policy":     ` conditional((context.reporter.kind | "inbound") == "outbound", "unknown", conditional(connection.mtls | false, "mutual_tls", "none"))`,
	}
}

func mixerVolumes(serviceAccount string) []apiv1.Volume {
	return []apiv1.Volume{
		{
			Name: "istio-certs",
			VolumeSource: apiv1.VolumeSource{
				Secret: &apiv1.SecretVolumeSource{
					SecretName: fmt.Sprintf("istio.%s", serviceAccount),
					Optional:   util.BoolPointer(true),
				},
			},
		},
		{
			Name: "uds-socket",
			VolumeSource: apiv1.VolumeSource{
				EmptyDir: &apiv1.EmptyDirVolumeSource{},
			},
		},
	}
}

func mixerContainer(policy bool, ns string) apiv1.Container {
	c := apiv1.Container{
		Name:            "mixer",
		Image:           "docker.io/istio/mixer:1.0.5",
		ImagePullPolicy: apiv1.PullIfNotPresent,
		Ports: []apiv1.ContainerPort{
			{ContainerPort: 9093},
			{ContainerPort: 42422},
		},
		Args: []string{
			"--address",
			"unix:///sock/mixer.socket",
			"--configStoreURL=k8s://",
			fmt.Sprintf("--configDefaultNamespace=%s", ns),
			"--trace_zipkin_url=http://zipkin:9411/api/v1/spans",
		},
		Env: []apiv1.EnvVar{
			{Name: "GODEBUG", Value: "gctrace=2"},
		},
		Resources: defaultResources(),
		VolumeMounts: []apiv1.VolumeMount{
			{Name: "uds-socket", MountPath: "/sock"},
		},
		LivenessProbe: &apiv1.Probe{
			Handler: apiv1.Handler{
				HTTPGet: &apiv1.HTTPGetAction{
					Path: "/version",
					Port: intstr.FromInt(9093),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       5,
		},
	}
	if policy {
		c.Args = append(c.Args, "--numCheckCacheEntries=0")
	}
	return c
}

func istioProxyContainer(mixer string) apiv1.Container {
	return apiv1.Container{
		Name:            "istio-proxy",
		Image:           "docker.io/istio/proxyv2:1.0.5",
		ImagePullPolicy: apiv1.PullIfNotPresent,
		Ports: []apiv1.ContainerPort{
			{ContainerPort: 9091},
			{ContainerPort: 15004},
			{ContainerPort: 15090, Protocol: apiv1.ProtocolTCP, Name: "http-envoy-prom"},
		},
		Args: []string{
			"proxy",
			"--serviceCluster",
			fmt.Sprintf("istio-%s", mixer),
			"--templateFile",
			fmt.Sprintf("/etc/istio/proxy/envoy_%s.yaml.tmpl", mixer),
			"--controlPlaneAuthPolicy",
			"NONE",
		},
		Env:       istioProxyEnv(),
		Resources: defaultResources(),
		VolumeMounts: []apiv1.VolumeMount{
			{
				Name:      "istio-certs",
				MountPath: "/etc/certs",
				ReadOnly:  true,
			},
			{
				Name:      "uds-socket",
				MountPath: "/sock",
			},
		},
	}
}

func mixerService(mixer, ns string) *apiv1.Service {
	svc := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("istio-%s", mixer),
			Namespace: ns,
			Labels: map[string]string{
				"istio": "mixer",
			},
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{Name: "grpc-mixer", Port: 9091},
				{Name: "grpc-mixer-mtls", Port: 15004},
				{Name: "http-monitoring", Port: 9093},
			},
			Selector: map[string]string{
				"istio":            "mixer",
				"istio-mixer-type": mixer,
			},
		},
	}
	if mixer == "telemetry" {
		svc.Spec.Ports = append(svc.Spec.Ports, apiv1.ServicePort{Name: "prometheus", Port: 42422})
	}
	return svc
}

func mixerAutoscaler(mixer, ns string) *autoscalev2beta1.HorizontalPodAutoscaler {
	return &autoscalev2beta1.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("istio-%s-autoscaler", mixer),
			Namespace: ns,
		},
		Spec: autoscalev2beta1.HorizontalPodAutoscalerSpec{
			MaxReplicas: 5,
			MinReplicas: util.IntPointer(1),
			ScaleTargetRef: autoscalev2beta1.CrossVersionObjectReference{
				Name:       fmt.Sprintf("istio-%s-deployment", mixer),
				Kind:       "Deployment",
				APIVersion: "apps/v1",
			},
			Metrics: targetAvgCpuUtil80(),
		},
	}
}