/*
Copyright 2017 The Kubernetes Authors.

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

// This file was copied from the main k/k repo and defaultResourcesToWeightMap was added.
// See: https://github.com/kubernetes/kubernetes/blob/release-1.19/pkg/scheduler/framework/plugins/noderesources/resource_allocation.go

package LoadAware

import (
	"context"
	"encoding/json"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	schedutil "k8s.io/kubernetes/pkg/scheduler/util"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
)

// resourceToWeightMap contains resource name and weight.
type resourceToWeightMap map[v1.ResourceName]int64

// defaultResourcesToWeightMap is used to set default resourceToWeight map for CPU and memory.
// The base unit for CPU is millicore, while the base using for memory is a byte.
// The default CPU weight is 1<<20 and default memory weight is 1. That means a millicore
// has a weighted score equivalent to 1 MiB.
var defaultResourcesToWeightMap = resourceToWeightMap{v1.ResourceMemory: 1, v1.ResourceCPU: 1 << 20, v1.ResourcePodNum: 1}

// resourceAllocationScorer contains information to calculate resource allocation score.
type resourceAllocationScorer struct {
	Name                string
	scorer              func(requested, allocatable resourceToValueMap) int64
	resourceToWeightMap resourceToWeightMap
	metricsClient       *metricsclient.Clientset
}

// resourceToValueMap contains resource name and score.
type resourceToValueMap map[v1.ResourceName]int64

// score will use `scorer` function to calculate the score.
func (r *resourceAllocationScorer) score(
	nodeInfo *framework.NodeInfo) (int64, *framework.Status) {
	node := nodeInfo.Node()
	if node == nil {
		return 0, framework.NewStatus(framework.Error, "node not found")
	}
	if r.resourceToWeightMap == nil {
		return 0, framework.NewStatus(framework.Error, "resources not found")
	}
	cost := make(resourceToValueMap, len(r.resourceToWeightMap))
	allocatable := make(resourceToValueMap, len(r.resourceToWeightMap))
	for resource := range r.resourceToWeightMap {
		allocatable[resource], cost[resource] = r.calculateResourceAllocatableCost(nodeInfo, resource)
	}
	score := r.scorer(cost, allocatable)

	allocatableJson, _ := json.Marshal(allocatable)
	costJson, _ := json.Marshal(cost)
	klog.Infof(" nodeName: %s  score: %d", node.Name, score)
	klog.Infof(" allocatable: %s cost: %s", string(allocatableJson), string(costJson))
	return score, nil
}

// calculateResourceAllocatableRequest returns resources Allocatable and Requested values
func (r *resourceAllocationScorer) calculateResourceAllocatableCost(nodeInfo *framework.NodeInfo, resource v1.ResourceName) (int64, int64) {
	if r.metricsClient == nil {
		return calculateResourceAllocatableRequest(nodeInfo, resource)
	}
	nodeMetrics, err := r.metricsClient.MetricsV1beta1().NodeMetricses().Get(context.TODO(), nodeInfo.Node().Name, metav1.GetOptions{})
	if err != nil {
		klog.InfoS("Could not get node metrics from metrics server, using requests instead", "node", nodeInfo.Node().Name, "err", err)
		return calculateResourceAllocatableRequest(nodeInfo, resource)
	}

	switch resource {
	case v1.ResourceCPU:
		if nodeMetrics.Usage.Cpu().MilliValue() == 0 {
			klog.InfoS("Node CPU usage from metrics server is 0, using requests instead", "node", nodeInfo.Node().Name)
			return nodeInfo.Allocatable.MilliCPU, nodeInfo.NonZeroRequested.MilliCPU
		}
		return nodeInfo.Allocatable.MilliCPU, nodeMetrics.Usage.Cpu().MilliValue()
	case v1.ResourceMemory:
		if nodeMetrics.Usage.Memory().Value() == 0 {
			klog.InfoS("Node memory usage from metrics server is 0, using requests instead", "node", nodeInfo.Node().Name)
			return nodeInfo.Allocatable.Memory, nodeInfo.NonZeroRequested.Memory
		}
		return nodeInfo.Allocatable.Memory, nodeMetrics.Usage.Memory().Value()
	case v1.ResourcePodNum:
		return int64(nodeInfo.Allocatable.AllowedPodNumber), int64(len(nodeInfo.Pods))
	case v1.ResourceEphemeralStorage:
		return nodeInfo.Allocatable.EphemeralStorage, nodeInfo.Requested.EphemeralStorage
	default:
		if schedutil.IsScalarResourceName(resource) {
			return nodeInfo.Allocatable.ScalarResources[resource], (nodeInfo.Requested.ScalarResources[resource])
		}
	}

	klog.InfoS("Requested resource not considered for node score calculation", "resource", resource)

	return 0, 0
}

func calculateResourceAllocatableRequest(nodeInfo *framework.NodeInfo, resource v1.ResourceName) (int64, int64) {
	switch resource {
	case v1.ResourceCPU:
		return nodeInfo.Allocatable.MilliCPU, nodeInfo.NonZeroRequested.MilliCPU
	case v1.ResourceMemory:
		return nodeInfo.Allocatable.Memory, nodeInfo.NonZeroRequested.Memory
	case v1.ResourcePodNum:
		return int64(nodeInfo.Allocatable.AllowedPodNumber), int64(len(nodeInfo.Pods))
	case v1.ResourceEphemeralStorage:
		return nodeInfo.Allocatable.EphemeralStorage, nodeInfo.NonZeroRequested.EphemeralStorage
	default:
		if schedutil.IsScalarResourceName(resource) {
			return nodeInfo.Allocatable.ScalarResources[resource], nodeInfo.NonZeroRequested.ScalarResources[resource]
		}
	}

	klog.InfoS("Requested resource not considered for node score calculation", "resource", resource)

	return 0, 0
}
