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
	"time"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	mcspv1 "github.com/VARSHITHA-P123/mcsp-operator/api/v1"
)

const (
	customerFinalizerName = "mcsp.mcsp.io/finalizer"
)

// MCPSCustomerReconciler reconciles a MCPSCustomer object
type MCPSCustomerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=mcsp.mcsp.io,resources=mcpscustomers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mcsp.mcsp.io,resources=mcpscustomers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mcsp.mcsp.io,resources=mcpscustomers/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy.open-cluster-management.io,resources=policies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy.open-cluster-management.io,resources=placementbindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=external-secrets.io,resources=externalsecrets,verbs=get;list;watch;create;update;patch;delete

func (r *MCPSCustomerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Step 1 — Get the MCPSCustomer CR
	mcpsCustomer := &mcspv1.MCPSCustomer{}
	err := r.Get(ctx, req.NamespacedName, mcpsCustomer)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("MCPSCustomer not found, might have been deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	customerName := mcpsCustomer.Spec.CustomerName
	log.Info("Reconciling MCPSCustomer", "customerName", customerName)

	// Handle deletion with finalizer
	if mcpsCustomer.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(mcpsCustomer, customerFinalizerName) {
			log.Info("Adding finalizer to MCPSCustomer", "customerName", customerName)
			controllerutil.AddFinalizer(mcpsCustomer, customerFinalizerName)
			if err := r.Update(ctx, mcpsCustomer); err != nil {
				log.Error(err, "Failed to add finalizer")
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
	} else {
		if controllerutil.ContainsFinalizer(mcpsCustomer, customerFinalizerName) {
			log.Info("MCPSCustomer is being deleted, starting cleanup", "customerName", customerName)

			if err := r.cleanupCustomerResources(ctx, customerName, log); err != nil {
				log.Error(err, "Failed to cleanup customer resources")
				return ctrl.Result{RequeueAfter: 10 * time.Second}, err
			}

			controllerutil.RemoveFinalizer(mcpsCustomer, customerFinalizerName)
			if err := r.Update(ctx, mcpsCustomer); err != nil {
				log.Error(err, "Failed to remove finalizer")
				return ctrl.Result{}, err
			}

			log.Info("MCPSCustomer deletion completed", "customerName", customerName)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, nil
	}

	// Step 2 — Create RHACM Policy
	rhacmPolicy := &unstructured.Unstructured{}
	rhacmPolicy.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "policy.open-cluster-management.io",
		Version: "v1",
		Kind:    "Policy",
	})
	err = r.Get(ctx, types.NamespacedName{
		Name:      customerName + "-policy",
		Namespace: "mcsp-platform",
	}, rhacmPolicy)
	if err != nil && errors.IsNotFound(err) {
		rhacmPolicy = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "policy.open-cluster-management.io/v1",
				"kind":       "Policy",
				"metadata": map[string]interface{}{
					"name":      customerName + "-policy",
					"namespace": "mcsp-platform",
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
										// Namespace
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
										// ResourceQuota
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
														"requests.cpu":    "4",
														"requests.memory": "8Gi",
														"limits.cpu":      "8",
														"limits.memory":   "16Gi",
														"pods":            "50",
													},
												},
											},
										},
										// LimitRange - auto assigns default limits to pods
										map[string]interface{}{
											"complianceType": "musthave",
											"objectDefinition": map[string]interface{}{
												"apiVersion": "v1",
												"kind":       "LimitRange",
												"metadata": map[string]interface{}{
													"name":      customerName + "-limits",
													"namespace": customerName,
												},
												"spec": map[string]interface{}{
													"limits": []interface{}{
														map[string]interface{}{
															"type": "Container",
															"default": map[string]interface{}{
																"cpu":    "500m",
																"memory": "512Mi",
															},
															"defaultRequest": map[string]interface{}{
																"cpu":    "100m",
																"memory": "128Mi",
															},
														},
													},
												},
											},
										},
										// RoleBinding
										map[string]interface{}{
											"complianceType": "musthave",
											"objectDefinition": map[string]interface{}{
												"apiVersion": "rbac.authorization.k8s.io/v1",
												"kind":       "RoleBinding",
												"metadata": map[string]interface{}{
													"name":      customerName + "-image-puller",
													"namespace": "mcsp-platform",
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
		log.Info("RHACM Policy created", "customer", customerName)
	}

	// Step 2b — Create PlacementBinding
	placementBinding := &unstructured.Unstructured{}
	placementBinding.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "policy.open-cluster-management.io",
		Version: "v1",
		Kind:    "PlacementBinding",
	})
	err = r.Get(ctx, types.NamespacedName{
		Name:      customerName + "-policy-binding",
		Namespace: "mcsp-platform",
	}, placementBinding)
	if err != nil && errors.IsNotFound(err) {
		placementBinding = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "policy.open-cluster-management.io/v1",
				"kind":       "PlacementBinding",
				"metadata": map[string]interface{}{
					"name":      customerName + "-policy-binding",
					"namespace": "mcsp-platform",
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
		log.Info("PlacementBinding created", "customer", customerName)
	}

	// Step 3 — Wait for namespace to be ready
	namespace := &corev1.Namespace{}
	err = r.Get(ctx, types.NamespacedName{Name: customerName}, namespace)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Waiting for namespace to be created by RHACM", "customerName", customerName)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Step 4 — Create Deployment Job
	job := &batchv1.Job{}
	err = r.Get(ctx, types.NamespacedName{
		Name:      customerName + "-deploy-job",
		Namespace: "mcsp-platform",
	}, job)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating deployment job", "customerName", customerName)

		ttl := int32(3600)
		backoffLimit := int32(3)

		job = &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      customerName + "-deploy-job",
				Namespace: "mcsp-platform",
				Labels: map[string]string{
					"app":    "mcsp-deployer",
					"tenant": customerName,
				},
			},
			Spec: batchv1.JobSpec{
				TTLSecondsAfterFinished: &ttl,
				BackoffLimit:            &backoffLimit,
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app":    "mcsp-deployer",
							"tenant": customerName,
						},
					},
					Spec: corev1.PodSpec{
						ServiceAccountName: "mcsp-deployer",
						RestartPolicy:      corev1.RestartPolicyOnFailure,
						Containers: []corev1.Container{
							{
								Name:    "deployer",
								Image:   "image-registry.openshift-image-registry.svc:5000/mcsp-platform/mcsp-deployer:latest",
								Command: []string{"/bin/bash", "-c"},
								Args: []string{
									fmt.Sprintf(`
										git clone https://$(GIT_TOKEN)@github.ibm.com/Manzanita/zps-mcsp-deploy.git -b mcsp-demo-2 /tmp/deploy &&
										cd /tmp/deploy &&
										sh deploy.sh %s
									`, customerName),
								},
								Env: []corev1.EnvVar{
									{
										Name: "GIT_TOKEN",
										ValueFrom: &corev1.EnvVarSource{
											SecretKeyRef: &corev1.SecretKeySelector{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: "ibm-github-token",
												},
												Key: "token",
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
		err = r.Create(ctx, job)
		if err != nil {
			log.Error(err, "Failed to create deployment job")
			return ctrl.Result{}, err
		}
		log.Info("Deployment job created", "customer", customerName)
	}

	// Step 5 — Update Status
	mcpsCustomer.Status.Deployed = true
	mcpsCustomer.Status.Message = fmt.Sprintf("Customer %s deployment job started", customerName)
	mcpsCustomer.Status.URL = fmt.Sprintf("https://mcsp-app-%s.apps.zps-mcsp-cluster.cp.fyre.ibm.com", customerName)
	err = r.Status().Update(ctx, mcpsCustomer)
	if err != nil {
		log.Error(err, "Failed to update MCPSCustomer status")
		return ctrl.Result{}, err
	}

	log.Info("MCPSCustomer reconciled successfully", "customerName", customerName)
	return ctrl.Result{}, nil
}

// cleanupCustomerResources performs cleanup of all resources created for a customer
func (r *MCPSCustomerReconciler) cleanupCustomerResources(ctx context.Context, customerName string, log logr.Logger) error {
	log.Info("Starting cleanup for customer", "customerName", customerName)

	// Step 1: Check if namespace exists and is not already terminating
	namespace := &corev1.Namespace{}
	err := r.Get(ctx, types.NamespacedName{Name: customerName}, namespace)
	namespaceExists := err == nil
	namespaceTerminating := namespaceExists && namespace.Status.Phase == corev1.NamespaceTerminating

	// Step 2: Create cleanup job to delete resources inside namespace (only if namespace exists and not terminating)
	if namespaceExists && !namespaceTerminating {
		log.Info("Creating cleanup job to delete resources in namespace", "customerName", customerName)
		
		cleanupJob := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      customerName + "-cleanup-job",
				Namespace: "mcsp-platform",
			},
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						ServiceAccountName: "mcsp-operator-controller-manager",
						RestartPolicy:      corev1.RestartPolicyNever,
						Containers: []corev1.Container{
							{
								Name:  "cleanup",
								Image: "registry.redhat.io/openshift4/ose-cli:latest",
								Command: []string{
									"/bin/bash",
									"-c",
									fmt.Sprintf(`
										echo "Starting cleanup for namespace %s"
										
										# Delete all deployments
										oc delete deployments --all -n %s --ignore-not-found=true --wait=false
										
										# Delete all statefulsets
										oc delete statefulsets --all -n %s --ignore-not-found=true --wait=false
										
										# Delete all services
										oc delete services --all -n %s --ignore-not-found=true --wait=false
										
										# Delete all configmaps (except kube-root-ca.crt and openshift-service-ca.crt)
										oc delete configmaps --all -n %s --ignore-not-found=true --wait=false --field-selector metadata.name!=kube-root-ca.crt,metadata.name!=openshift-service-ca.crt
										
										# Delete all secrets (except default service account tokens)
										oc delete secrets --all -n %s --ignore-not-found=true --wait=false --field-selector type!=kubernetes.io/service-account-token
										
										# Wait for pods to terminate
										echo "Waiting for pods to terminate..."
										for i in {1..30}; do
											POD_COUNT=$(oc get pods -n %s --no-headers 2>/dev/null | wc -l)
											if [ "$POD_COUNT" -eq "0" ]; then
												echo "All pods terminated"
												break
											fi
											echo "Waiting... $POD_COUNT pods remaining"
											sleep 2
										done
										
										echo "Cleanup job completed"
									`, customerName, customerName, customerName, customerName, customerName, customerName, customerName),
								},
							},
						},
					},
				},
				BackoffLimit: ptr.To(int32(3)),
			},
		}

		// Try to create cleanup job
		err = r.Get(ctx, types.NamespacedName{Name: customerName + "-cleanup-job", Namespace: "mcsp-platform"}, &batchv1.Job{})
		if errors.IsNotFound(err) {
			if err := r.Create(ctx, cleanupJob); err != nil {
				log.Error(err, "Failed to create cleanup job, will proceed with namespace deletion anyway")
			} else {
				log.Info("Cleanup job created, waiting for completion", "customerName", customerName)
				
				// Wait for cleanup job to complete (max 2 minutes)
				for i := 0; i < 60; i++ {
					job := &batchv1.Job{}
					err := r.Get(ctx, types.NamespacedName{Name: customerName + "-cleanup-job", Namespace: "mcsp-platform"}, job)
					if err == nil {
						if job.Status.Succeeded > 0 {
							log.Info("Cleanup job completed successfully", "customerName", customerName)
							break
						}
						if job.Status.Failed > 0 {
							log.Info("Cleanup job failed, proceeding with namespace deletion", "customerName", customerName)
							break
						}
					}
					time.Sleep(2 * time.Second)
				}
			}
		}
	} else if namespaceTerminating {
		log.Info("Namespace already terminating, skipping cleanup job creation", "customerName", customerName)
	}

	// Step 3: Delete deployment job
	job := &batchv1.Job{}
	err = r.Get(ctx, types.NamespacedName{Name: customerName + "-deploy-job", Namespace: "mcsp-platform"}, job)
	if err == nil {
		if err := r.Delete(ctx, job); err != nil && !errors.IsNotFound(err) {
			log.Error(err, "Failed to delete deployment job")
		} else {
			log.Info("Deployment job deleted", "customerName", customerName)
		}
	}

	// Step 4: Delete cleanup job
	cleanupJob := &batchv1.Job{}
	err = r.Get(ctx, types.NamespacedName{Name: customerName + "-cleanup-job", Namespace: "mcsp-platform"}, cleanupJob)
	if err == nil {
		if err := r.Delete(ctx, cleanupJob); err != nil && !errors.IsNotFound(err) {
			log.Error(err, "Failed to delete cleanup job")
		} else {
			log.Info("Cleanup job deleted", "customerName", customerName)
		}
	}

	// Step 5: Delete PlacementBinding
	placementBinding := &unstructured.Unstructured{}
	placementBinding.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "policy.open-cluster-management.io",
		Version: "v1",
		Kind:    "PlacementBinding",
	})
	err = r.Get(ctx, types.NamespacedName{Name: customerName + "-policy-binding", Namespace: "mcsp-platform"}, placementBinding)
	if err == nil {
		if err := r.Delete(ctx, placementBinding); err != nil && !errors.IsNotFound(err) {
			log.Error(err, "Failed to delete PlacementBinding")
			return err
		}
		log.Info("PlacementBinding deleted", "customerName", customerName)
	}

	// Step 6: Delete RHACM Policy
	rhacmPolicy := &unstructured.Unstructured{}
	rhacmPolicy.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "policy.open-cluster-management.io",
		Version: "v1",
		Kind:    "Policy",
	})
	err = r.Get(ctx, types.NamespacedName{Name: customerName + "-policy", Namespace: "mcsp-platform"}, rhacmPolicy)
	if err == nil {
		if err := r.Delete(ctx, rhacmPolicy); err != nil && !errors.IsNotFound(err) {
			log.Error(err, "Failed to delete RHACM Policy")
			return err
		}
		log.Info("RHACM Policy deleted", "customerName", customerName)
	}

	// Step 7: Delete Namespace and wait for complete deletion
	err = r.Get(ctx, types.NamespacedName{Name: customerName}, namespace)
	if err == nil {
		if err := r.Delete(ctx, namespace); err != nil && !errors.IsNotFound(err) {
			log.Error(err, "Failed to delete Namespace")
			return err
		}
		log.Info("Namespace deletion initiated", "customerName", customerName)

		// Wait for namespace to be fully deleted (max 5 minutes)
		for i := 0; i < 150; i++ {
			err = r.Get(ctx, types.NamespacedName{Name: customerName}, namespace)
			if errors.IsNotFound(err) {
				log.Info("Namespace fully deleted", "customerName", customerName)
				break
			}
			if i%10 == 0 {
				log.Info("Still waiting for namespace deletion", "customerName", customerName, "iteration", i)
			}
			time.Sleep(2 * time.Second)
		}

		// Final check
		err = r.Get(ctx, types.NamespacedName{Name: customerName}, namespace)
		if err == nil {
			log.Error(fmt.Errorf("namespace still exists after timeout"), "Namespace deletion timed out", "customerName", customerName)
			return fmt.Errorf("namespace %s deletion timed out after 5 minutes", customerName)
		}
	}

	log.Info("Cleanup completed successfully", "customerName", customerName)
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MCPSCustomerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mcspv1.MCPSCustomer{}).
		Named("mcpscustomer").
		Complete(r)
}
