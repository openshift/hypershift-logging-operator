/*
	 Copyright 2023.

	 Licensed under the Apache License, Version 2.0 (the "License");
	 you may not use this file except in compliance with the License.
	 You may obtain a copy of the License at

		http:www.apache.org/licenses/LICENSE-2.0

	 Unless required by applicable law or agreed to in writing, software
	 distributed under the License is distributed on an "AS IS" BASIS,
	 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	 See the License for the specific language governing permissions and
	 limitations under the License.
*/
package clusterlogforwardertemplate

import (
	"context"
	"testing"

	"github.com/go-logr/logr/testr"
	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"

	hlov1alpha1 "github.com/openshift/hypershift-logging-operator/api/v1alpha1"
	"github.com/openshift/hypershift-logging-operator/pkg/constants"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"

	configv1 "github.com/openshift/api/config/v1"
	hyperv1beta1 "github.com/openshift/hypershift/api/v1beta1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuildClusterLogForwarder(t *testing.T) {
	const namespace = "test-ns"

	tests := []struct {
		name                        string
		clusterLogForwarderTemplate *hlov1alpha1.ClusterLogForwarderTemplate
		expectedClusterLogForwarder *loggingv1.ClusterLogForwarder
		expectErr                   bool
		request                     ctrl.Request
		hcpList                     []hyperv1beta1.HostedControlPlane
	}{
		{
			name: "should create new ClusterLogForwarder in given namespace",
			clusterLogForwarderTemplate: &hlov1alpha1.ClusterLogForwarderTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "instance",
					Namespace: constants.OperatorNamespace,
				},
				Spec: hlov1alpha1.ClusterLogForwarderTemplateSpec{

					Template: loggingv1.ClusterLogForwarderSpec{
						ServiceAccountName: "test-sa",
					},
				},
			},
			expectedClusterLogForwarder: &loggingv1.ClusterLogForwarder{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "instance",
					Namespace: constants.OperatorNamespace,
				},
			},
			expectErr: false,
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "instance",
					Namespace: namespace,
				},
			},
			hcpList: []hyperv1beta1.HostedControlPlane{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "name1",
						Namespace: namespace,
					},
				},
			},
		},
		{
			name: "should not create ClusterLogForwarder for empty hosted clusters",
			clusterLogForwarderTemplate: &hlov1alpha1.ClusterLogForwarderTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "instance",
					Namespace: namespace,
				},
				Spec: hlov1alpha1.ClusterLogForwarderTemplateSpec{

					Template: loggingv1.ClusterLogForwarderSpec{
						ServiceAccountName: "test-sa",
					},
				},
			},
			expectedClusterLogForwarder: nil,
			expectErr:                   false,
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "instance",
					Namespace: namespace,
				},
			},
			hcpList: []hyperv1beta1.HostedControlPlane{},
		},
		{
			name:                        "should not create ClusterLogForwarder for empty ClusterLogForwarderTemplate",
			clusterLogForwarderTemplate: nil,
			expectedClusterLogForwarder: nil,
			expectErr:                   false,
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "instance",
					Namespace: namespace,
				},
			},
			hcpList: []hyperv1beta1.HostedControlPlane{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "name1",
						Namespace: namespace,
					},
				},
			},
		},
	}

	for _, test := range tests {

		t.Run(test.name, func(t *testing.T) {
			c := NewTestMock(t).Client
			//create template if exist
			if test.clusterLogForwarderTemplate != nil {
				c.Create(context.TODO(), test.clusterLogForwarderTemplate)
			}

			r := &ClusterLogForwarderTemplateReconciler{
				Client:  c,
				Scheme:  c.Scheme(),
				log:     testr.New(t),
				hcpList: test.hcpList,
			}

			_, err := r.Reconcile(context.TODO(), test.request)
			if err != nil {
				if !test.expectErr {
					t.Errorf("expected no err, got %v", err)
				}
				return
			}

			if test.expectErr {
				t.Error("expected err, got nil")
			}
			if !test.expectErr {
				if test.expectedClusterLogForwarder != nil {
					for _, hcp := range r.hcpList {
						clusterLogForwarder := new(loggingv1.ClusterLogForwarder)
						if err := r.Get(context.TODO(), client.ObjectKey{
							Namespace: hcp.Namespace,
							Name:      "instance",
						}, clusterLogForwarder); err != nil {
							t.Fatalf("unexpected err: %v", err)
						}
					}

				}
			}
		})
	}
}

type MockKubeClient struct {
	Client client.Client
}

func NewTestMock(t *testing.T, objs ...client.Object) *MockKubeClient {
	mock, err := NewMock(objs...)
	if err != nil {
		t.Fatal(err)
	}

	return mock
}

func NewMock(obs ...client.Object) (*MockKubeClient, error) {
	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		return nil, err
	}

	if err := configv1.Install(s); err != nil {
		return nil, err
	}

	if err := hyperv1beta1.AddToScheme(s); err != nil {
		return nil, err
	}

	if err := loggingv1.AddToScheme(s); err != nil {
		return nil, err
	}

	if err := hlov1alpha1.AddToScheme(s); err != nil {
		return nil, err
	}

	return &MockKubeClient{
		Client: fake.NewClientBuilder().WithScheme(s).WithObjects(obs...).Build(),
	}, nil
}
