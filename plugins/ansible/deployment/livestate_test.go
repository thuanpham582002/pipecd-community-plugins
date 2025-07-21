// Copyright 2025 The PipeCD Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package deployment

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/pipe-cd/community-plugins/plugins/ansible/config"
	sdk "github.com/pipe-cd/piped-plugin-sdk-go"
	"go.uber.org/zap"
)

func TestPlugin_GetLivestate(t *testing.T) {
	// Create temporary directory with test files
	tempDir := t.TempDir()
	
	// Create test playbook
	playbookContent := `---
- name: Test playbook
  hosts: localhost
  tasks:
    - name: Test task
      debug:
        msg: "Hello World"
`
	playbookPath := filepath.Join(tempDir, "test-playbook.yml")
	err := os.WriteFile(playbookPath, []byte(playbookContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test playbook: %v", err)
	}

	// Create test inventory
	inventoryContent := `[local]
localhost ansible_connection=local
`
	inventoryPath := filepath.Join(tempDir, "inventory")
	err = os.WriteFile(inventoryPath, []byte(inventoryContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test inventory: %v", err)
	}

	plugin := &Plugin{}
	cfg := &config.AnsiblePluginConfig{}
	
	// Create test deploy target
	dts := []*sdk.DeployTarget[config.AnsibleDeployTargetConfig]{
		{
			Name: "test-target",
			Config: config.AnsibleDeployTargetConfig{
				AnsiblePath: "ansible-playbook", // Use system ansible if available
			},
		},
	}

	// Create application config
	appConfig := &sdk.ApplicationConfig[config.AnsibleApplicationSpec]{
		Spec: &config.AnsibleApplicationSpec{
			Playbook: config.AnsiblePlaybookManifest{
				Path:      "test-playbook.yml",
				Inventory: "inventory",
				Verbosity: 1,
			},
		},
	}

	// Create test input
	input := &sdk.GetLivestateInput[config.AnsibleApplicationSpec]{
		Request: sdk.GetLivestateRequest[config.AnsibleApplicationSpec]{
			PipedID:         "test-piped",
			ApplicationID:   "test-app",
			ApplicationName: "test-application",
			DeploymentSource: sdk.DeploymentSource[config.AnsibleApplicationSpec]{
				ApplicationDirectory: tempDir,
				CommitHash:          "abc123",
				ApplicationConfig:   appConfig,
			},
		},
		Logger: zap.NewNop(),
	}

	ctx := context.Background()
	
	// Test GetLivestate
	response, err := plugin.GetLivestate(ctx, cfg, dts, input)
	
	// Verify response structure (even if ansible is not available)
	if err != nil {
		t.Fatalf("GetLivestate returned error: %v", err)
	}
	
	if response == nil {
		t.Fatal("GetLivestate returned nil response")
	}
	
	// Verify live state structure
	if len(response.LiveState.Resources) == 0 {
		t.Error("Expected at least one resource in live state")
	}
	
	// Verify the first resource
	resource := response.LiveState.Resources[0]
	if resource.ResourceType != "ansible-playbook" {
		t.Errorf("Expected resource type 'ansible-playbook', got %s", resource.ResourceType)
	}
	
	if resource.DeployTarget != "test-target" {
		t.Errorf("Expected deploy target 'test-target', got %s", resource.DeployTarget)
	}
	
	// Verify sync state structure
	if response.SyncState.Status == sdk.ApplicationSyncStateUnknown {
		t.Log("Sync state is unknown (expected if ansible is not available)")
	}
	
	t.Logf("Resource health status: %v", resource.HealthStatus)
	t.Logf("Resource health description: %s", resource.HealthDescription)
	t.Logf("Sync state status: %v", response.SyncState.Status)
	t.Logf("Sync state reason: %s", response.SyncState.ShortReason)
}

func TestPlugin_GetLivestate_ErrorHandling(t *testing.T) {
	plugin := &Plugin{}
	cfg := &config.AnsiblePluginConfig{}
	
	// Test with invalid deployment source (missing app config)
	input := &sdk.GetLivestateInput[config.AnsibleApplicationSpec]{
		Request: sdk.GetLivestateRequest[config.AnsibleApplicationSpec]{
			PipedID:         "test-piped",
			ApplicationID:   "test-app",
			ApplicationName: "test-application",
			DeploymentSource: sdk.DeploymentSource[config.AnsibleApplicationSpec]{
				ApplicationDirectory: "/nonexistent",
				CommitHash:          "abc123",
			},
		},
		Logger: zap.NewNop(),
	}

	ctx := context.Background()
	
	// This should return an error due to missing app config
	_, err := plugin.GetLivestate(ctx, cfg, nil, input)
	if err == nil {
		t.Error("Expected GetLivestate to return error for invalid deployment source")
	}
}