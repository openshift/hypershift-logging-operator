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

package hostedcluster

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/openshift/hypershift-logging-operator/api/v1alpha1"
	"github.com/openshift/hypershift-logging-operator/controllers/hypershiftlogforwarder"
	"github.com/openshift/hypershift-logging-operator/pkg/hostedcluster"
	hyperv1beta1 "github.com/openshift/hypershift/api/v1beta1"
)

var hostedClusters = map[string]hypershiftlogforwarder.HostedCluster{}

// ClusterLogForwarderTemplateReconciler reconciles a ClusterLogForwarderTemplate object
type HosteClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	log    logr.Logger
}

//+kubebuilder:rbac:groups=logging.managed.openshift.io,resources=clusterlogforwardertemplates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=logging.managed.openshift.io,resources=clusterlogforwardertemplates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=logging.managed.openshift.io,resources=clusterlogforwardertemplates/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *HosteClusterReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (ctrl.Result, error) {
	r.log = ctrllog.FromContext(ctx).WithName("controller")

	hostedCluster := &hyperv1beta1.HostedCluster{}
	if err := r.Get(ctx, req.NamespacedName, hostedCluster); err != nil {
		// Ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification).
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// check hosted cluster status, if it's new created and ready, start the reconcile
	newReadyCluster := true

	_, exist := hostedClusters[hostedCluster.Name]

	if !exist {
		if newReadyCluster {
			restConfig, err := hostedcluster.BuildGuestKubeConfig(r.Client, req.NamespacedName, r.log)
			if err != nil {
				setupLog.Error(err, "getting guest cluster kubeconfig")
			}

			hsCluster, err := cluster.New(restConfig)
			if err != nil {
				setupLog.Error(err, "creating guest cluster kubeconfig")
			}

			hostedCluster := hypershiftlogforwarder.HostedCluster{
				Cluster:      hsCluster,
				HCPNamespace: hcpNamespace,
			}
			hostedClusters[hcp.Name] = hostedCluster
			r := hypershiftlogforwarder.HyperShiftLogForwarderReconciler{
				Client:       hostedCluster.Cluster.GetClient(),
				Scheme:       mgr.GetScheme(),
				MCClient:     mgr.GetClient(),
				HCPNamespace: hostedCluster.HCPNamespace,
			}

			err := ctrl.NewControllerManagedBy(mgr).
				For(&v1alpha1.HyperShiftLogForwarder{}).
				Watches(
					source.NewKindWithCache(&v1alpha1.HyperShiftLogForwarder{}, hostedCluster.Cluster.GetCache()),
					&handler.EnqueueRequestForObject{},
				).
				Complete(&r)
			if err != nil {

				return &r, err
			}

		}

	}

	//if it's deleted, stop the reconcile

	return ctrl.Result{}, nil
}

func eventPredicates() predicate.Predicate {
	return predicate.Funcs{
		DeleteFunc: func(e event.DeleteEvent) bool {
			return true
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *HosteClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hyperv1beta1.HostedCluster{}).
		WithEventFilter(eventPredicates()).
		Complete(r)
}
