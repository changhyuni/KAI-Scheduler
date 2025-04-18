// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package skiptopowner

import (
	"context"
	"fmt"
	"maps"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgroup"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins"
)

type skipTopOwnerGrouper struct {
	client         client.Client
	supportedTypes map[metav1.GroupVersionKind]plugins.GetPodGroupMetadataFunc
}

func NewSkipTopOwnerGrouper(client client.Client, supportedTypes map[metav1.GroupVersionKind]plugins.GetPodGroupMetadataFunc) *skipTopOwnerGrouper {
	return &skipTopOwnerGrouper{
		client:         client,
		supportedTypes: supportedTypes,
	}
}

func (sk *skipTopOwnerGrouper) GetPodGroupMetadata(
	skippedOwner *unstructured.Unstructured, pod *v1.Pod, otherOwners ...*metav1.PartialObjectMetadata,
) (*podgroup.Metadata, error) {
	var lastOwnerPartial *metav1.PartialObjectMetadata
	if len(otherOwners) <= 1 {
		lastOwnerPartial = &metav1.PartialObjectMetadata{
			TypeMeta:   pod.TypeMeta,
			ObjectMeta: pod.ObjectMeta,
		}
	} else {
		lastOwnerPartial = otherOwners[len(otherOwners)-2]
	}

	lastOwner, err := sk.getObjectInstance(lastOwnerPartial)
	if err != nil {
		return nil, fmt.Errorf("failed to get last owner: %w", err)
	}

	if lastOwner.GetLabels() == nil {
		lastOwner.SetLabels(skippedOwner.GetLabels())
	} else {
		maps.Copy(lastOwner.GetLabels(), skippedOwner.GetLabels())
	}
	return sk.getSupportedTypePGMetadata(lastOwner, pod, otherOwners[:len(otherOwners)-1]...)
}

func (sk *skipTopOwnerGrouper) getSupportedTypePGMetadata(
	lastOwner *unstructured.Unstructured, pod *v1.Pod, otherOwners ...*metav1.PartialObjectMetadata,
) (*podgroup.Metadata, error) {
	ownerKind := metav1.GroupVersionKind(lastOwner.GroupVersionKind())
	if function, found := sk.supportedTypes[ownerKind]; found {
		return function(lastOwner, pod, otherOwners...)
	}
	return plugins.GetPodGroupMetadata(lastOwner, pod, otherOwners...)
}

func (sk *skipTopOwnerGrouper) getObjectInstance(objectRef *metav1.PartialObjectMetadata) (*unstructured.Unstructured, error) {
	topOwnerInstance := &unstructured.Unstructured{}
	topOwnerInstance.SetGroupVersionKind(objectRef.GroupVersionKind())
	key := types.NamespacedName{
		Namespace: objectRef.GetNamespace(),
		Name:      objectRef.GetName(),
	}
	err := sk.client.Get(context.Background(), key, topOwnerInstance)
	return topOwnerInstance, err
}
