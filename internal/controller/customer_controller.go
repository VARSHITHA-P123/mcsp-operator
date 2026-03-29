/*
Copyright 2026.
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
package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	mcspv1 "github.com/VARSHITHA-P123/mcsp-operator/api/v1"
)

// CustomerReconciler reconciles a Customer object
type CustomerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=mcsp.mcsp.io,resources=customers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mcsp.mcsp.io,resources=customers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mcsp.mcsp.io,resources=customers/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy.open-cluster-management.io,resources=policies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy.open-cluster-management.io,resources=placementbindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=external-secrets.io,resources=externalsecrets,verbs=get;list;watch;create;update;patch;delete

func (r *CustomerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// TODO: Confirm with Gin backend teammate which namespace
	// Customer CR will be created in
	// Currently assuming: default namespace

	// Step 1 — Get the Customer CR
	customer := &mcspv1.Customer{}
	err := r.Get(ctx, req.NamespacedName, customer)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Customer not found, might have been deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	customerName := customer.Spec.CustomerName
	log.Info("Reconciling Customer", "customerName", customerName)

	// Step 2 — Create RHACM Policy
	// This tells RHACM to create the namespace
	// with resource quotas and RBAC permissions
	// RHACM handles namespace creation — not us!
	rhacmPolicy := &unstructured.Unstructured{}
	rhacmPolicy.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "policy.open-cluster-management.io",
		Version: "v1",
		Kind:    "Policy",
	})
	err = r.Get(ctx, types.NamespacedName{
		Name:      customerName + "-policy",
		Namespace: "learning-workspace",
	}, rhacmPolicy)
	if err != nil && errors.IsNotFound(err) {
		rhacmPolicy = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "policy.open-cluster-management.io/v1",
				"kind":       "Policy",
				"metadata": map[string]interface{}{
					"name":      customerName + "-policy",
					"namespace": "learning-workspace",
				},
				"spec": map[string]interface{}{
					"remediationAction": "enforce",
					"disabled":          false,
					"policy-templates": []interface{}{
						map[string]interface{}{
							"objectDefinition": map[string]interface{}{
								"apiVersion": "policy.open-cluster-management.io/v1",
								"kind":       "ConfigurationPolicy",
								"metadata": map[string]interface{}{
									"name": customerName + "-namespace",
								},
								"spec": map[string]interface{}{
									"remediationAction": "enforce",
									"severity":          "low",
									"object-templates": []interface{}{
										map[string]interface{}{
											"complianceType": "musthave",
											"objectDefinition": map[string]interface{}{
												"apiVersion": "v1",
												"kind":       "Namespace",
												"metadata": map[string]interface{}{
													"name": customerName,
													"labels": map[string]interface{}{
														"tenant":   customerName,
														"customer": "true",
													},
												},
											},
										},
										map[string]interface{}{
											"complianceType": "musthave",
											"objectDefinition": map[string]interface{}{
												"apiVersion": "v1",
												"kind":       "ResourceQuota",
												"metadata": map[string]interface{}{
													"name":      customerName + "-quota",
													"namespace": customerName,
												},
												"spec": map[string]interface{}{
													"hard": map[string]interface{}{
														"requests.cpu":    "1",
														"requests.memory": "512Mi",
														"limits.cpu":      "2",
														"limits.memory":   "1Gi",
														"pods":            "10",
													},
												},
											},
										},
										map[string]interface{}{
											"complianceType": "musthave",
											"objectDefinition": map[string]interface{}{
												"apiVersion": "rbac.authorization.k8s.io/v1",
												"kind":       "RoleBinding",
												"metadata": map[string]interface{}{
													"name":      customerName + "-image-puller",
													"namespace": "learning-workspace",
												},
												"roleRef": map[string]interface{}{
													"apiGroup": "rbac.authorization.k8s.io",
													"kind":     "ClusterRole",
													"name":     "system:image-puller",
												},
												"subjects": []interface{}{
													map[string]interface{}{
														"kind":      "ServiceAccount",
														"name":      "default",
														"namespace": customerName,
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		err = r.Create(ctx, rhacmPolicy)
		if err != nil {
			log.Error(err, "Failed to create RHACM policy")
			return ctrl.Result{}, err
		}
		log.Info("RHACM Policy created — namespace will be provisioned by RHACM", "customer", customerName)
	}

	// Step 2b — Create PlacementBinding for RHACM Policy
	// This tells RHACM which cluster to apply the policy to
	placementBinding := &unstructured.Unstructured{}
	placementBinding.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "policy.open-cluster-management.io",
		Version: "v1",
		Kind:    "PlacementBinding",
	})
	err = r.Get(ctx, types.NamespacedName{
		Name:      customerName + "-policy-binding",
		Namespace: "learning-workspace",
	}, placementBinding)
	if err != nil && errors.IsNotFound(err) {
		placementBinding = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "policy.open-cluster-management.io/v1",
				"kind":       "PlacementBinding",
				"metadata": map[string]interface{}{
					"name":      customerName + "-policy-binding",
					"namespace": "learning-workspace",
				},
				"placementRef": map[string]interface{}{
					"name":     "mcsp-hello-world-placement",
					"apiGroup": "cluster.open-cluster-management.io",
					"kind":     "Placement",
				},
				"subjects": []interface{}{
					map[string]interface{}{
						"name":     customerName + "-policy",
						"apiGroup": "policy.open-cluster-management.io",
						"kind":     "Policy",
					},
				},
			},
		}
		err = r.Create(ctx, placementBinding)
		if err != nil {
			log.Error(err, "Failed to create PlacementBinding")
			return ctrl.Result{}, err
		}
		log.Info("PlacementBinding created — RHACM will now enforce policy", "customer", customerName)
	}

	// Step 3 — Create Certificate using Cert Manager
	// Cert Manager handles TLS — not us!
	certificate := &unstructured.Unstructured{}
	certificate.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "cert-manager.io",
		Version: "v1",
		Kind:    "Certificate",
	})
	err = r.Get(ctx, types.NamespacedName{
		Name:      customerName + "-tls",
		Namespace: customerName,
	}, certificate)
	if err != nil && errors.IsNotFound(err) {
		certificate = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "cert-manager.io/v1",
				"kind":       "Certificate",
				"metadata": map[string]interface{}{
					"name":      customerName + "-tls",
					"namespace": customerName,
				},
				"spec": map[string]interface{}{
					"secretName": customerName + "-tls",
					"issuerRef": map[string]interface{}{
						"name": "mcsp-selfsigned-issuer",
						"kind": "ClusterIssuer",
					},
					"dnsNames": []interface{}{
						fmt.Sprintf("mcsp-app-%s.apps.zps-mcsp1.cp.fyre.ibm.com", customerName),
					},
					"duration":    "2160h",
					"renewBefore": "360h",
				},
			},
		}
		err = r.Create(ctx, certificate)
		if err != nil {
			log.Error(err, "Failed to create Certificate")
			return ctrl.Result{}, err
		}
		log.Info("Certificate created — Cert Manager will provision TLS", "customer", customerName)
	}

	// Step 4 — Create ExternalSecret
	// External Secrets handles credential injection — not us!
	externalSecret := &unstructured.Unstructured{}
	externalSecret.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "external-secrets.io",
		Version: "v1",
		Kind:    "ExternalSecret",
	})
	err = r.Get(ctx, types.NamespacedName{
		Name:      customerName + "-secrets",
		Namespace: customerName,
	}, externalSecret)
	if err != nil && errors.IsNotFound(err) {
		externalSecret = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "external-secrets.io/v1",
				"kind":       "ExternalSecret",
				"metadata": map[string]interface{}{
					"name":      customerName + "-secrets",
					"namespace": customerName,
				},
				"spec": map[string]interface{}{
					"refreshInterval": "1h",
					"secretStoreRef": map[string]interface{}{
						"name": "kubernetes-secret-store",
						"kind": "ClusterSecretStore",
					},
					"target": map[string]interface{}{
						"name":           customerName + "-secrets",
						"creationPolicy": "Owner",
					},
					"data": []interface{}{
						map[string]interface{}{
							"secretKey": "customerName",
							"remoteRef": map[string]interface{}{
								// TODO: Confirm secret naming convention with teammates
								"key":      customerName + "-source-secret",
								"property": "customerName",
							},
						},
						map[string]interface{}{
							"secretKey": "apiKey",
							"remoteRef": map[string]interface{}{
								"key":      customerName + "-source-secret",
								"property": "apiKey",
							},
						},
						map[string]interface{}{
							"secretKey": "dbPassword",
							"remoteRef": map[string]interface{}{
								"key":      customerName + "-source-secret",
								"property": "dbPassword",
							},
						},
					},
				},
			},
		}
		err = r.Create(ctx, externalSecret)
		if err != nil {
			log.Error(err, "Failed to create ExternalSecret")
			return ctrl.Result{}, err
		}
		log.Info("ExternalSecret created — External Secrets will inject credentials", "customer", customerName)
	}

	// Step 5 — Create Deployment
	// Default replicas = 1
	// TODO: Update when replicas field is added to CR
	replicas := int32(1)
	deployment := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{
		Name:      "mcsp-app",
		Namespace: customerName,
	}, deployment)
	if err != nil && errors.IsNotFound(err) {
		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mcsp-app",
				Namespace: customerName,
				Labels: map[string]string{
					"app":    "mcsp-app",
					"tenant": customerName,
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "mcsp-app",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app":    "mcsp-app",
							"tenant": customerName,
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "mcsp-app",
								Image: "image-registry.openshift-image-registry.svc:5000/learning-workspace/mcsp-hello-world:latest",
								Ports: []corev1.ContainerPort{
									{
										ContainerPort: 8080,
									},
								},
								Env: []corev1.EnvVar{
									{
										Name:  "PORT",
										Value: "8080",
									},
									{
										Name:  "SCENARIO",
										Value: "2 of 3",
									},
									{
										Name:  "NAMESPACE",
										Value: customerName,
									},
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("50m"),
										corev1.ResourceMemory: resource.MustParse("64Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("100m"),
										corev1.ResourceMemory: resource.MustParse("128Mi"),
									},
								},
							},
						},
					},
				},
			},
		}
		err = r.Create(ctx, deployment)
		if err != nil {
			log.Error(err, "Failed to create deployment")
			return ctrl.Result{}, err
		}
		log.Info("Deployment created", "customer", customerName)
	}

	// Step 6 — Create Service
	service := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{
		Name:      "mcsp-app",
		Namespace: customerName,
	}, service)
	if err != nil && errors.IsNotFound(err) {
		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mcsp-app",
				Namespace: customerName,
				Labels: map[string]string{
					"app":    "mcsp-app",
					"tenant": customerName,
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"app": "mcsp-app",
				},
				Ports: []corev1.ServicePort{
					{
						Name:       "http",
						Port:       80,
						TargetPort: intstr.FromInt(8080),
						Protocol:   corev1.ProtocolTCP,
					},
				},
				Type: corev1.ServiceTypeClusterIP,
			},
		}
		err = r.Create(ctx, service)
		if err != nil {
			log.Error(err, "Failed to create service")
			return ctrl.Result{}, err
		}
		log.Info("Service created", "customer", customerName)
	}

	// Step 7 — Create Route
	route := &unstructured.Unstructured{}
	route.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "route.openshift.io",
		Version: "v1",
		Kind:    "Route",
	})
	err = r.Get(ctx, types.NamespacedName{
		Name:      "mcsp-app",
		Namespace: customerName,
	}, route)
	if err != nil && errors.IsNotFound(err) {
		route = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "route.openshift.io/v1",
				"kind":       "Route",
				"metadata": map[string]interface{}{
					"name":      "mcsp-app",
					"namespace": customerName,
					"labels": map[string]interface{}{
						"app":    "mcsp-app",
						"tenant": customerName,
					},
				},
				"spec": map[string]interface{}{
					"to": map[string]interface{}{
						"kind": "Service",
						"name": "mcsp-app",
					},
					"port": map[string]interface{}{
						"targetPort": "http",
					},
					"tls": map[string]interface{}{
						"termination":                   "edge",
						"insecureEdgeTerminationPolicy": "Redirect",
					},
				},
			},
		}
		err = r.Create(ctx, route)
		if err != nil {
			log.Error(err, "Failed to create route")
			return ctrl.Result{}, err
		}
		log.Info("Route created", "customer", customerName)
	}

	// Step 8 — Update Status
	customer.Status.Deployed = true
	customer.Status.Message = fmt.Sprintf("Customer %s successfully deployed", customerName)
	// TODO: Confirm URL format with teammates
	customer.Status.URL = fmt.Sprintf("https://mcsp-app-%s.apps.zps-mcsp1.cp.fyre.ibm.com", customerName)
	err = r.Status().Update(ctx, customer)
	if err != nil {
		log.Error(err, "Failed to update customer status")
		return ctrl.Result{}, err
	}

	log.Info("Customer reconciled successfully", "customerName", customerName)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CustomerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mcspv1.Customer{}).
		Named("customer").
		Complete(r)
}
