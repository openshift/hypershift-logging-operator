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

package hypershiftlogforwarder

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/openshift/hypershift-logging-operator/api/v1alpha1"
	"github.com/openshift/hypershift-logging-operator/pkg/clusterlogforwarder"
	"github.com/openshift/hypershift-logging-operator/pkg/consts"
	hyperv1beta1 "github.com/openshift/hypershift/api/v1beta1"
)

var (
	nonSupportTypeCondition = loggingv1.Condition{
		Type:    "Degraded",
		Status:  "True",
		Reason:  "NonSupportedInput",
		Message: "The input supports only the audit type",
	}
	illegalNameCondition = loggingv1.Condition{
		Type:    "Degraded",
		Status:  "True",
		Reason:  "IllegalName",
		Message: fmt.Sprintf("The Name contains the SRE preserved string %s", consts.ProviderManagedRuleNamePrefix),
	}
	incorrectInstanceNameCondition = loggingv1.Condition{
		Type:    "Degraded",
		Status:  "True",
		Reason:  "NonSupportResourceName",
		Message: fmt.Sprintf("The name of the HyperShiftLogForwarder must be '%s'", consts.SingletonName),
	}
)

// HostedCluster keeps hosted cluster info
type HostedCluster struct {
	Cluster      cluster.Cluster
	ClusterId    string
	HCPNamespace string
}

// HyperShiftLogForwarderReconciler reconciles a HyperShiftLogForwarder object
type HyperShiftLogForwarderReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	MCClient     client.Client
	HCPNamespace string
	log          logr.Logger
}

//+kubebuilder:rbac:groups=logging.managed.openshift.io,resources=hypershiftlogforwarders,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=logging.managed.openshift.io,resources=hypershiftlogforwarders/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=logging.managed.openshift.io,resources=hypershiftlogforwarders/finalizers,verbs=update

// NewLogForwarderReconcilerReconciler ...
func NewHyperShiftLogForwarderReconciler(mgr ctrl.Manager, hostedClusters map[string]HostedCluster) (*HyperShiftLogForwarderReconciler, error) {
	r := HyperShiftLogForwarderReconciler{
		Scheme: mgr.GetScheme(),
	}

	for _, hostedCluster := range hostedClusters {
		r := HyperShiftLogForwarderReconciler{
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

	return &r, nil
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *HyperShiftLogForwarderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	log := logr.Logger{}.WithName("hyperShiftLogForwarder-controller")

	instance := &v1alpha1.HyperShiftLogForwarder{}

	if r.HCPNamespace == "" {
		return ctrl.Result{}, nil
	}

	err := r.Get(ctx, req.NamespacedName, instance)
	if errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	if req.NamespacedName.Name != consts.SingletonName {
		log.V(3).Info("hyperShiftLogForwarder is singleton and name should be 'instance'")
		instance.Status.Conditions.SetCondition(incorrectInstanceNameCondition)
		err = r.Status().Update(ctx, instance)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	clf := &loggingv1.ClusterLogForwarder{}

	clfFound := false
	err = r.MCClient.Get(context.TODO(), types.NamespacedName{Name: "instance", Namespace: r.HCPNamespace}, clf)
	if err != nil && errors.IsNotFound(err) {
		clfFound = false
	} else if err == nil {
		clfFound = true
	} else {
		return ctrl.Result{}, err
	}

	deletion := false
	// HyperShiftLogForwarder was deleted by CU
	if !instance.DeletionTimestamp.IsZero() {
		deletion = true
		if controllerutil.ContainsFinalizer(instance, consts.ManagedLoggingFinalizer) {
			controllerutil.RemoveFinalizer(instance, consts.ManagedLoggingFinalizer)
			if err = r.Update(ctx, instance); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if !controllerutil.ContainsFinalizer(instance, consts.ManagedLoggingFinalizer) {
			controllerutil.AddFinalizer(instance, consts.ManagedLoggingFinalizer)
			if err = r.Update(ctx, instance); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	if err = r.ValidateInputs(instance); err != nil {
		return ctrl.Result{}, err
	}

	if err = r.ValidateOutputs(instance); err != nil {
		return ctrl.Result{}, err
	}

	if err = r.ValidatePipelines(instance); err != nil {
		return ctrl.Result{}, err
	}

	instance.Status.Conditions.RemoveCondition(illegalNameCondition.Type)
	instance.Status.Conditions.RemoveCondition(nonSupportTypeCondition.Type)
	instance.Status.Conditions.RemoveCondition(incorrectInstanceNameCondition.Type)
	if err = r.Status().Update(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	clusterlogforwarder.CleanUpClusterLogForwarder(clf, consts.CustomerManagedRuleNamePrefix)

	if deletion {
		err = r.MCClient.Update(ctx, clf)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else {
		clf.Namespace = r.HCPNamespace
		clf.Name = "instance"
		clf = clusterlogforwarder.BuildInputsFromHLF(instance, clf)
		clf = clusterlogforwarder.BuildOutputsFromHLF(instance, clf)
		clf = clusterlogforwarder.BuildPipelinesFromHLF(instance, clf)

		if clfFound {
			if err = r.MCClient.Update(ctx, clf); err != nil {
				return ctrl.Result{}, err
			}
		} else {
			if err = r.MCClient.Create(ctx, clf); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HyperShiftLogForwarderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hyperv1beta1.HostedCluster{}).
		// WithEventFilter(eventPredicates()).
		Complete(r)
}

// ValidateInputs validates HLF inputs
func (r *HyperShiftLogForwarderReconciler) ValidateInputs(hlf *v1alpha1.HyperShiftLogForwarder) error {
	for _, input := range hlf.Spec.Inputs {
		if input.Infrastructure != nil || input.Application != nil {
			r.log.V(3).Info("support only audit log for HyperShiftLogForwarder")
			hlf.Status.Conditions.SetCondition(nonSupportTypeCondition)
			if err := r.Status().Update(context.TODO(), hlf); err != nil {
				return err
			}
			return fmt.Errorf("support only audit log for HyperShiftLogForwarder")
		}
		if strings.Contains(input.Name, consts.ProviderManagedRuleNamePrefix) {
			r.log.V(3).Info(fmt.Sprintf("preserved string %s cannot be set to the input name", consts.ProviderManagedRuleNamePrefix))
			hlf.Status.Conditions.SetCondition(illegalNameCondition)
			if err := r.Status().Update(context.TODO(), hlf); err != nil {
				return err
			}
			return fmt.Errorf("preserved string %s cannot be set to the input name", consts.ProviderManagedRuleNamePrefix)
		}
	}
	return nil
}

// ValidateOutputs validates the HLF outputs
func (r *HyperShiftLogForwarderReconciler) ValidateOutputs(hlf *v1alpha1.HyperShiftLogForwarder) error {
	for _, output := range hlf.Spec.Outputs {
		if strings.Contains(output.Name, consts.ProviderManagedRuleNamePrefix) {
			r.log.V(3).Info(fmt.Sprintf("preserved string %s cannot be set to the output name", consts.ProviderManagedRuleNamePrefix))
			hlf.Status.Conditions.SetCondition(illegalNameCondition)
			if err := r.Status().Update(context.TODO(), hlf); err != nil {
				return err
			}
			return fmt.Errorf("preserved string %s cannot be set to the output name", consts.ProviderManagedRuleNamePrefix)
		}
	}
	return nil
}

// ValidatePipelines validates the HLF pipelines
func (r *HyperShiftLogForwarderReconciler) ValidatePipelines(hlf *v1alpha1.HyperShiftLogForwarder) error {
	for _, ppl := range hlf.Spec.Pipelines {
		for _, ir := range ppl.InputRefs {
			if ir == "application" || ir == "infrastructure" {
				r.log.V(3).Info("support only audit log for HyperShiftLogForwarder")
				hlf.Status.Conditions.SetCondition(nonSupportTypeCondition)
				if err := r.Status().Update(context.TODO(), hlf); err != nil {
					return err
				}
				return fmt.Errorf("support only audit log for HyperShiftLogForwarder")
			}
		}
		if strings.Contains(ppl.Name, consts.ProviderManagedRuleNamePrefix) {
			r.log.V(3).Info(fmt.Sprintf("preserved string %s cannot be set to the pipeline name", consts.ProviderManagedRuleNamePrefix))
			hlf.Status.Conditions.SetCondition(illegalNameCondition)
			if err := r.Status().Update(context.TODO(), hlf); err != nil {
				return err
			}
			return fmt.Errorf("preserved string %s cannot be set to the pipeline name", consts.ProviderManagedRuleNamePrefix)
		}
	}
	return nil
}
