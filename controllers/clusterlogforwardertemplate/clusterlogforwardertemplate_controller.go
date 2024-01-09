/*
Copyright 2023.

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

package clusterlogforwardertemplate

import (
	"context"
	"reflect"

	"github.com/go-logr/logr"
	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	hyperv1beta1 "github.com/openshift/hypershift/api/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/source"

	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	hlov1alpha1 "github.com/openshift/hypershift-logging-operator/api/v1alpha1"
	"github.com/openshift/hypershift-logging-operator/pkg/clusterlogforwarder"
	"github.com/openshift/hypershift-logging-operator/pkg/constants"
	"github.com/openshift/hypershift-logging-operator/pkg/hostedcluster"
)

// ClusterLogForwarderTemplateReconciler reconciles a ClusterLogForwarderTemplate object
type ClusterLogForwarderTemplateReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	log     logr.Logger
	hcpList []hyperv1beta1.HostedControlPlane
}

//+kubebuilder:rbac:groups=logging.managed.openshift.io,resources=clusterlogforwardertemplates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=logging.managed.openshift.io,resources=clusterlogforwardertemplates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=logging.managed.openshift.io,resources=clusterlogforwardertemplates/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ClusterLogForwarderTemplateReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (ctrl.Result, error) {

	var err error
	r.log = ctrllog.FromContext(ctx).WithName("controller")

	if len(r.hcpList) == 0 {
		r.hcpList, err = hostedcluster.GetHostedControlPlanes(r.Client, ctx, false)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	template := &hlov1alpha1.ClusterLogForwarderTemplate{}

	// Reconcile the CLFT resource in the operator namespace
	if err := r.Get(ctx, types.NamespacedName{Namespace: constants.OperatorNamespace, Name: req.Name}, template); err != nil {
		// Ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification).
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	deletion := false

	if !template.ObjectMeta.DeletionTimestamp.IsZero() {
		deletion = true
		controllerutil.RemoveFinalizer(template, constants.ManagedLoggingFinalizer)
		err = r.Client.Update(ctx, template)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else {
		controllerutil.AddFinalizer(template, constants.ManagedLoggingFinalizer)
		err = r.Client.Update(ctx, template)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	for _, hcp := range r.hcpList {

		// Declare the CLF resource in each iteration
		clf := &loggingv1.ClusterLogForwarder{}

		found := false
		err = r.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: hcp.Namespace}, clf)
		if errors.IsNotFound(err) {
			found = false
		} else if err != nil {
			return ctrl.Result{}, err
		} else {
			found = true
		}
		// If CLFT is deleted, and the CLF exists in the HCP namespace, do clean up
		if deletion && found {
			err = r.Delete(ctx, clf)
			if err != nil {
				return ctrl.Result{}, err
			}
		}

		// If CLFT is not deleting, recreate the CLF in the HCP namespace
		if !deletion {
			r.log.V(1).Info("Status", "Deletion", false, "Found", found)

			// Build the CLF from the current template
			newClf := r.buildClusterLogForwarder(template, hcp.Namespace)

			if found {
				// If the existing CLF is the same as the new one, skip
				if reflect.DeepEqual(newClf.Spec, clf.Spec) {
					continue
				} else {
					// If the existing CLF is not the same as the new built one, delete existing
					err = r.Delete(ctx, clf)
					if err != nil {
						return ctrl.Result{}, err
					}
				}
			}
			err = r.Create(ctx, newClf)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

// buildClusterLogForwarder build ClusterLogForwarder from the ClusterLogForwarderTemplate
func (r *ClusterLogForwarderTemplateReconciler) buildClusterLogForwarder(
	template *hlov1alpha1.ClusterLogForwarderTemplate,
	ns string,
) *loggingv1.ClusterLogForwarder {

	clf := &loggingv1.ClusterLogForwarder{}

	clf.Name = template.Name
	clf.Namespace = ns

	clfBuilder := clusterlogforwarder.ClusterLogForwarderBuilder{
		Clf:  clf,
		Clft: template,
	}

	clfBuilder.BuildInputsFromTemplate().
		BuildOutputsFromTemplate().
		BuildPipelinesFromTemplate().
		BuildFiltersFromTemplate().
		BuildServiceAccount()

	return clfBuilder.Clf
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterLogForwarderTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hlov1alpha1.ClusterLogForwarderTemplate{}).
		Watches(&source.Kind{Type: &hyperv1beta1.HostedControlPlane{}}, &enqueueRequestForHostedControlPlane{Client: mgr.GetClient()}).
		Complete(r)
}
