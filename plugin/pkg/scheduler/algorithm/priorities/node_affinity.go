/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package priorities

import (
	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/plugin/pkg/scheduler/algorithm"
	schedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api"
)

type NodeAffinity struct {
	nodeLister algorithm.NodeLister
}

func NewNodeAffinityPriority(nodeLister algorithm.NodeLister) algorithm.PriorityFunction {
	nodeAffinity := &NodeAffinity{
		nodeLister: nodeLister,
	}
	return nodeAffinity.CalculateNodeAffinityPriority
}

// compute a sum by iterating through the elements of PreferredDuringSchedulingIgnoredDuringExecution
// and adding "weight" to the sum if the node matches the corresponding MatchExpressions; the
// node(s) with the highest sum are the most preferred.
func (s *NodeAffinity) CalculateNodeAffinityPriority(pod *api.Pod, machinesToPods map[string][]*api.Pod, podLister algorithm.PodLister, nodeLister algorithm.NodeLister) (schedulerapi.HostPriorityList, error) {

	var maxCount int
	counts := map[string]int{}

	nodes, err := nodeLister.List()
	if err != nil {
		return nil, err
	}

	affinity, err := schedulerapi.GetAffinityFromPod(pod)
	if err != nil {
		return nil, err
	}

	// A nil element of PreferredDuringSchedulingIgnoredDuringExecution matches no objects
	// An element of PreferredDuringSchedulingIgnoredDuringExecution that refers to an empty PreferredSchedulingTerm matches all objects
	if affinity.NodeAffinity != nil && affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution != nil {
		// Match PreferredDuringSchedulingIgnoredDuringExecution term by term.
		for _, preferredSchedulingTerm := range affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
			if preferredSchedulingTerm.Weight == 0 {
				continue
			}

			nodeSelector, err := schedulerapi.NodeSelectorRequirementsAsSelector(preferredSchedulingTerm.MatchExpressions)
			if err != nil {
				return nil, err
			}

			for _, node := range nodes.Items {
				if nodeSelector.Matches(labels.Set(node.Labels)) {
					counts[node.Name] += preferredSchedulingTerm.Weight
				}

				if counts[node.Name] > maxCount {
					maxCount = counts[node.Name]
				}
			}
		}
	}

	result := []schedulerapi.HostPriority{}
	for _, node := range nodes.Items {
		fScore := float32(0)
		if maxCount > 0 {
			fScore = 10 * (float32(counts[node.Name]) / float32(maxCount))
		}
		result = append(result, schedulerapi.HostPriority{Host: node.Name, Score: int(fScore)})
		glog.V(10).Infof(
			"%v -> %v: NodeAffinityPriority, Score: (%d)", pod.Name, node.Name, int(fScore),
		)
	}
	return result, nil
}
