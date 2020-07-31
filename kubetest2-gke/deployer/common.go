/*
Copyright 2020 The Kubernetes Authors.

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

package deployer

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"k8s.io/klog"
	"sigs.k8s.io/kubetest2/pkg/boskos"
)

const (
	gkeProjectResourceType = "gke-project"
)

func (d *deployer) init() error {
	var err error
	d.doInit.Do(func() { err = d.initialize() })
	return err
}

// initialize should only be called by init(), behind a sync.Once
func (d *deployer) initialize() error {
	if d.commonOptions.ShouldUp() {
		if err := d.verifyUpFlags(); err != nil {
			return fmt.Errorf("init failed to verify flags for up: %s", err)
		}

		if len(d.projects) == 0 {
			klog.V(1).Info("No GCP projects provided, acquiring from Boskos %d project/s", d.boskosProjectsRequested)

			boskosClient, err := boskos.NewClient(d.boskosLocation)
			if err != nil {
				return fmt.Errorf("failed to make boskos client: %s", err)
			}
			d.boskos = boskosClient

			for i := 0; i < d.boskosProjectsRequested; i++ {
				resource, err := boskos.Acquire(
					d.boskos,
					gkeProjectResourceType,
					time.Duration(d.boskosAcquireTimeoutSeconds)*time.Second,
					d.boskosHeartbeatClose,
				)

				if err != nil {
					return fmt.Errorf("init failed to get project from boskos: %s", err)
				}
				d.projects = append(d.projects, resource.Name)
				klog.V(1).Infof("Got project %s from boskos", resource.Name)
			}
		}

		// Multi-cluster name adjustment
		numProjects := len(d.projects)
		d.clustersLayout = make(map[string][]string, numProjects)
		if numProjects > 1 {
			for iProject, project := range d.projects {
				var err error
				clusters, err := filterMultiProjectClusterName(d.clusters, iProject)
				if err != nil {
					return fmt.Errorf("aborting initialization due to invalid project name: %s", err)
				}
				d.clustersLayout[project] = clusters
			}
		} else {
			// Backwards compatible construction
			d.clustersLayout = make(map[string][]string, 1)
			d.clustersLayout[d.projects[0]] = []string{d.clusters[0]}
		}
	}

	if d.commonOptions.ShouldDown() {
		if err := d.verifyDownFlags(); err != nil {
			return fmt.Errorf("init failed to verify flags for down: %s", err)
		}
	}

	return nil
}

func filterMultiProjectClusterName(clusters []string, projectID int) ([]string, error) {
	result := make([]string, 0)
	for _, clusterName := range clusters {
		parts := strings.Split(clusterName, "-")
		if len(parts) != 2 {
			return nil, fmt.Errorf("cluster name does not follow expected format (name-projectIndex): %s", clusterName)
		}
		projectIndex, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("cluster name does not follow contain a valid project index (name-projectIndex. E.g: cluster-0): %s", err)
		}
		if projectIndex == projectID {
			// Network field is empty
			result = append(result, parts[0])
		}
	}
	return result, nil
}
