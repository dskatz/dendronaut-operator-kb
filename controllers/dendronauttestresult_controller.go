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

package controllers

import (
	"context"
	"sort"
	"strconv"

	batchv1alpha1 "github.com/dskatz/dendronaut-operator-kb/api/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// DendronautTestResultReconciler reconciles a DendronautTestResult object
type DendronautTestResultReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

var (
	dendronautTestRunsGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "dendronaut",
			Subsystem: "e2e",
			Name:      "results",
			Help:      "Dendronaut Test Results.",
		},
		[]string{"test_name", "pass", "message"},
	)
)

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(dendronautTestRunsGauge)
}

// +kubebuilder:rbac:groups=batch.dendronaut.example.com,resources=dendronauttestresults,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch.dendronaut.example.com,resources=dendronauttestresults/status,verbs=get;update;patch

func (r *DendronautTestResultReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	reqLogger := r.Log.WithValues("dendronauttestresult", req.NamespacedName)

	reqLogger.Info("Reconciling DendronautTestResult")

	instance := &batchv1alpha1.DendronautTestResult{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	// simply log the instance
	testName := instance.Spec.TestName
	passBool := instance.Spec.Pass
	message := instance.Spec.Message

	// Debug logging
	reqLogger.Info("TestResults", "testName", testName, "passBool", passBool, "message", message)

	if !instance.Status.Published {
		reqLogger.Info("Not Published", "testName", testName)

		// publish test results metrics to prometheus
		gaugeVal := float64(0)
		if passBool {
			gaugeVal = float64(1)
		}

		dendronautTestRunsGauge.With(prometheus.Labels{"test_name": testName, "pass": strconv.FormatBool(passBool), "message": message}).Set(gaugeVal)

		instance.Status.Published = true
		// Update Status
		if err := r.Status().Update(context.Background(), instance); err != nil {
			reqLogger.Error(err, "unable to update DendronautTestResult status.")
			return ctrl.Result{}, err
		}
	}

	// Only keep the last 10 DendronautTestResult objects

	var testResults batchv1alpha1.DendronautTestResultList
	reqLogger.Info("Fetching Tests", "Namespace", req.Namespace, "client", client.InNamespace(req.Namespace))
	if err := r.List(ctx, &testResults, client.InNamespace(req.Namespace)); err != nil {
		reqLogger.Error(err, "unable to list test results")
		return ctrl.Result{}, err
	}

	maxItems := 10
	if len(testResults.Items) >= maxItems {
		sort.Slice(testResults.Items, func(i, j int) bool {
			return testResults.Items[i].ObjectMeta.CreationTimestamp.Before(&testResults.Items[j].ObjectMeta.CreationTimestamp)
		})

		reqLogger.Info("TestResults Max Items", "Length", len(testResults.Items))

		for i, testResult := range testResults.Items {
			if int32(i) >= int32(len(testResults.Items))-int32(maxItems) {
				break
			}
			reqLogger.Info("Deleting old result", "i", i, "testResult.TestName", testResult.Spec.TestName)
			if err := r.Delete(ctx, &testResult, client.PropagationPolicy(metav1.DeletePropagationBackground)); ignoreNotFound(err) != nil {
				reqLogger.Error(err, "unable to delete old result", "result", testResult.ObjectMeta.Name)
			} else {
				reqLogger.Info("Deleted old result.", "result", testResult.ObjectMeta.Name)
			}
		}

	}

	return ctrl.Result{}, nil
}

/*
We generally want to ignore (not requeue) NotFound errors, since we'll get a
reconciliation request once the object exists, and requeuing in the meantime
won't help.
*/
func ignoreNotFound(err error) error {
	if apierrs.IsNotFound(err) {
		return nil
	}
	return err
}

func (r *DendronautTestResultReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1alpha1.DendronautTestResult{}).
		Complete(r)
}
