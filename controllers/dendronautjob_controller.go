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

	batchv1alpha1 "github.com/dskatz/dendronaut-operator-kb/api/v1alpha1"
	"github.com/go-logr/logr"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DendronautJobReconciler reconciles a DendronautJob object
type DendronautJobReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=batch.dendronaut.example.com,resources=dendronautjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch.dendronaut.example.com,resources=dendronautjobs/status,verbs=get;update;patch

func (r *DendronautJobReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	reqLogger := r.Log.WithValues("dendronautjob", req.NamespacedName)

	reqLogger.Info("Reconciling DendronautJob")

	instance := &batchv1alpha1.DendronautJob{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
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

	// your logic here
	// Check if this Pod already exists
	found := &batchv1beta1.CronJob{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Found Error ", "err", err)
		reqLogger.Info("Creating a new CronJob", "CronJob.Namespace", instance.Namespace, "CronJob.Name", instance.Name)
		job := r.newCronJob(instance)
		err = r.Client.Create(ctx, job)
		if err != nil {
			return ctrl.Result{}, err
		}

		// Pod created successfully - don't requeue
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	// Pod already exists - don't requeue
	reqLogger.Info("Skip reconcile: CronJob already exists", "CronJob.Namespace", found.Namespace, "CronJob.Name", found.Name)

	return ctrl.Result{}, nil
}

func (r *DendronautJobReconciler) newCronJob(cr *batchv1alpha1.DendronautJob) *batchv1beta1.CronJob {

	labels := map[string]string{
		"app": cr.Name,
	}
	job := &batchv1beta1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: cr.Spec.Cron,
	}

	ctrl.SetControllerReference(cr, job, r.Scheme)

	return job

}

var (
	jobOwnerKey = ".metadata.controller"
	apiGVStr    = batchv1alpha1.GroupVersion.String()
)

func (r *DendronautJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(&batchv1beta1.CronJob{}, jobOwnerKey, func(rawObj runtime.Object) []string {
		// grab the job object, extract the owner...
		job := rawObj.(*batchv1beta1.CronJob)
		owner := metav1.GetControllerOf(job)
		if owner == nil {
			return nil
		}
		// ...make sure it's a CronJob...
		if owner.APIVersion != apiGVStr || owner.Kind != "DendronautJob" {
			return nil
		}

		// ...and if so, return it
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1alpha1.DendronautJob{}).
		Owns(&batchv1beta1.CronJob{}).
		Complete(r)
}
