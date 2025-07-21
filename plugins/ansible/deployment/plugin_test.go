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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/pipe-cd/community-plugins/plugins/ansible/config"
	sdk "github.com/pipe-cd/piped-plugin-sdk-go"
)

func TestPlugin_FetchDefinedStages(t *testing.T) {
	p := &Plugin{}
	stages := p.FetchDefinedStages()

	assert.Len(t, stages, 1)
	assert.Contains(t, stages, "ANSIBLE_SYNC")
}

func TestPlugin_DetermineStrategy(t *testing.T) {
	p := &Plugin{}
	ctx := context.Background()
	cfg := &config.AnsiblePluginConfig{}

	t.Run("with pipeline stages", func(t *testing.T) {
		// Load test application config with pipeline stages
		appConfig := sdk.LoadApplicationConfigForTest[config.AnsibleApplicationSpec](t, "../examples/test-app/app.pipecd.yaml", "ansible")

		deploymentSource := &sdk.DeploymentSource[config.AnsibleApplicationSpec]{
			ApplicationConfig: appConfig,
		}

		input := &sdk.DetermineStrategyInput[config.AnsibleApplicationSpec]{
			Request: sdk.DetermineStrategyRequest[config.AnsibleApplicationSpec]{
				TargetDeploymentSource: *deploymentSource,
			},
		}

		resp, err := p.DetermineStrategy(ctx, cfg, input)
		require.NoError(t, err)
		assert.Equal(t, sdk.SyncStrategyPipelineSync, resp.Strategy)
	})

	t.Run("without pipeline stages", func(t *testing.T) {
		// Create a basic app config file without pipeline stages for testing
		basicConfig := `apiVersion: pipecd.dev/v1beta1
kind: Application
spec:
  name: ansible-test-app
  plugins:
    ansible:
      playbook:
        path: playbook.yml`

		// Write temp config file
		tmpFile := t.TempDir() + "/app.pipecd.yaml"
		require.NoError(t, os.WriteFile(tmpFile, []byte(basicConfig), 0644))

		appConfig := sdk.LoadApplicationConfigForTest[config.AnsibleApplicationSpec](t, tmpFile, "ansible")

		deploymentSource := &sdk.DeploymentSource[config.AnsibleApplicationSpec]{
			ApplicationConfig: appConfig,
		}

		input := &sdk.DetermineStrategyInput[config.AnsibleApplicationSpec]{
			Request: sdk.DetermineStrategyRequest[config.AnsibleApplicationSpec]{
				TargetDeploymentSource: *deploymentSource,
			},
		}

		resp, err := p.DetermineStrategy(ctx, cfg, input)
		require.NoError(t, err)
		assert.Equal(t, sdk.SyncStrategyPipelineSync, resp.Strategy)
	})
}

func TestBuildQuickSync(t *testing.T) {
	tests := []struct {
		name         string
		autoRollback bool
		expected     []sdk.QuickSyncStage
	}{
		{
			name:         "without auto rollback",
			autoRollback: false,
			expected: []sdk.QuickSyncStage{
				{
					Name:        "ANSIBLE_SYNC",
					Description: "Execute Ansible playbook",
					Metadata: map[string]string{
						sdk.MetadataKeyStageDisplay: "Deploy Ansible Configuration",
					},
					AvailableOperation: sdk.ManualOperationNone,
				},
			},
		},
		{
			name:         "with auto rollback",
			autoRollback: true,
			expected: []sdk.QuickSyncStage{
				{
					Name:        "ANSIBLE_SYNC",
					Description: "Execute Ansible playbook",
					Metadata: map[string]string{
						sdk.MetadataKeyStageDisplay: "Deploy Ansible Configuration",
					},
					AvailableOperation: sdk.ManualOperationNone,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildQuickSync(tt.autoRollback)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildPipeline(t *testing.T) {
	t.Run("with stages", func(t *testing.T) {
		stages := []sdk.StageConfig{
			{Name: "ANSIBLE_SYNC", Index: 0},
		}

		result, err := buildPipeline(stages, false, zap.NewNop())
		assert.NoError(t, err)

		expected := []sdk.PipelineStage{
			{
				Name:    "ANSIBLE_SYNC",
				Index:   0,
				Rollback: false,
				Metadata: map[string]string{
					sdk.MetadataKeyStageDisplay: "Deploy Ansible Configuration",
				},
				AvailableOperation: sdk.ManualOperationNone,
			},
		}

		assert.Equal(t, expected, result)
	})

	t.Run("without stages", func(t *testing.T) {
		// Empty stages should create a default ANSIBLE_SYNC stage
		stages := []sdk.StageConfig{}

		result, err := buildPipeline(stages, false, zap.NewNop())
		assert.NoError(t, err)

		expected := []sdk.PipelineStage{
			{
				Name:    "ANSIBLE_SYNC",
				Index:   0,
				Rollback: false,
				Metadata: map[string]string{
					sdk.MetadataKeyStageDisplay: "Deploy Ansible Configuration",
				},
				AvailableOperation: sdk.ManualOperationNone,
			},
		}

		assert.Equal(t, expected, result)
		assert.Len(t, result, 1)
	})
}
