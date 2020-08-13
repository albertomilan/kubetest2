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
	"os"
	"path/filepath"

	"k8s.io/klog"
	"sigs.k8s.io/kubetest2/pkg/exec"
)

func (d *deployer) Up() error {
	klog.V(1).Info("GCE deployer starting Up()")

	if err := d.init(); err != nil {
		return fmt.Errorf("up failed to init: %s", err)
	}

	if d.EnableComputeAPI {
		klog.V(2).Info("enabling compute API for project")
		if err := enableComputeAPI(d.GCPProject); err != nil {
			return fmt.Errorf("up couldn't enable compute API: %s", err)
		}
	}

	defer func() {
		if err := d.DumpClusterLogs(); err != nil {
			klog.Warningf("Dumping cluster logs at the end of Up() failed: %s", err)
		}
	}()

	env := d.buildEnv()
	script := filepath.Join(d.RepoRoot, "cluster", "kube-up.sh")
	klog.V(2).Infof("About to run script at: %s", script)

	cmd := exec.Command(script)
	cmd.SetEnv(env...)
	exec.InheritOutput(cmd)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error encountered during %s: %s", script, err)
	}

	isUp, err := d.IsUp()
	if err != nil {
		klog.Warningf("failed to check if cluster is up: %s", err)
	} else if isUp {
		klog.V(1).Infof("cluster reported as up")
	} else {
		klog.Errorf("cluster reported as down")
	}

	klog.V(2).Info("about to create nodeport firewall rule")
	if err := d.createFirewallRuleNodePort(); err != nil {
		return fmt.Errorf("failed to create firewall rule: %s", err)
	}

	klog.V(2).Info("about to create hostport firewall rule")
	if err := d.createFirewallRuleHostPort(); err != nil {
		return fmt.Errorf("failed to create firewall rule: %s", err)
	}

	return nil
}

func enableComputeAPI(project string) error {
	// In freshly created GCP projects, the compute API is
	// not enabled. We need it. Enabling it after it has
	// already been enabled is a relatively fast no-op,
	// so this can be called without consequence.

	env := os.Environ()
	cmd := exec.Command(
		"gcloud",
		"services",
		"enable",
		"compute.googleapis.com",
		"--project="+project,
	)
	cmd.SetEnv(env...)
	exec.InheritOutput(cmd)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to enable compute API: %s", err)
	}

	return nil
}

func (d *deployer) verifyUpFlags() error {
	if d.NumNodes < 1 {
		return fmt.Errorf("number of nodes must be at least 1")
	}

	if err := d.setRepoPathIfNotSet(); err != nil {
		return err
	}

	d.kubectlPath = filepath.Join(d.RepoRoot, "cluster", "kubectl.sh")

	// verifyUpFlags does not check for a gcp project because it is
	// assumed that one will be acquired from boskos if it is not set

	return nil
}
