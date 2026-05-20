package controller

import (
	"context"
	"fmt"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	registryv1alpha1 "github.com/ken/harbor-operator/api/v1alpha1"
)

const (
	ConditionAvailable   = "Available"
	ConditionProgressing = "Progressing"
	ConditionDegraded    = "Degraded"
)

func setProgressing(harbor *registryv1alpha1.Harbor, reason, msg string) {
	harbor.Status.Phase = registryv1alpha1.HarborPhaseReconciling
	apimeta.SetStatusCondition(&harbor.Status.Conditions, metav1.Condition{
		Type:               ConditionProgressing,
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		Message:            msg,
		ObservedGeneration: harbor.Generation,
	})
	apimeta.SetStatusCondition(&harbor.Status.Conditions, metav1.Condition{
		Type:               ConditionAvailable,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            msg,
		ObservedGeneration: harbor.Generation,
	})
	apimeta.SetStatusCondition(&harbor.Status.Conditions, metav1.Condition{
		Type:               ConditionDegraded,
		Status:             metav1.ConditionFalse,
		Reason:             "AsExpected",
		Message:            "",
		ObservedGeneration: harbor.Generation,
	})
}

func setReady(harbor *registryv1alpha1.Harbor, ready, total int32) {
	harbor.Status.Phase = registryv1alpha1.HarborPhaseReady
	harbor.Status.ReadyComponents = ready
	harbor.Status.TotalComponents = total
	harbor.Status.URL = "https://" + harbor.Spec.Hostname
	apimeta.SetStatusCondition(&harbor.Status.Conditions, metav1.Condition{
		Type:               ConditionAvailable,
		Status:             metav1.ConditionTrue,
		Reason:             "ComponentsReady",
		Message:            fmt.Sprintf("%d/%d components ready", ready, total),
		ObservedGeneration: harbor.Generation,
	})
	apimeta.SetStatusCondition(&harbor.Status.Conditions, metav1.Condition{
		Type:               ConditionProgressing,
		Status:             metav1.ConditionFalse,
		Reason:             "AsExpected",
		Message:            "",
		ObservedGeneration: harbor.Generation,
	})
	apimeta.SetStatusCondition(&harbor.Status.Conditions, metav1.Condition{
		Type:               ConditionDegraded,
		Status:             metav1.ConditionFalse,
		Reason:             "AsExpected",
		Message:            "",
		ObservedGeneration: harbor.Generation,
	})
}

func setDegraded(harbor *registryv1alpha1.Harbor, reason, msg string) {
	harbor.Status.Phase = registryv1alpha1.HarborPhaseFailed
	apimeta.SetStatusCondition(&harbor.Status.Conditions, metav1.Condition{
		Type:               ConditionDegraded,
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		Message:            msg,
		ObservedGeneration: harbor.Generation,
	})
	apimeta.SetStatusCondition(&harbor.Status.Conditions, metav1.Condition{
		Type:               ConditionAvailable,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            msg,
		ObservedGeneration: harbor.Generation,
	})
}

func updateStatus(ctx context.Context, c client.Client, harbor *registryv1alpha1.Harbor) error {
	harbor.Status.ObservedGeneration = harbor.Generation
	return c.Status().Update(ctx, harbor)
}
