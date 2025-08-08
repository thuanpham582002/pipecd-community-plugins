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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pipe-cd/community-plugins/plugins/ansible/config"
	sdk "github.com/pipe-cd/piped-plugin-sdk-go"
	"go.uber.org/zap"
)

// Plugin implements sdk.DeploymentPlugin and sdk.LivestatePlugin for Ansible.
type Plugin struct{}

var _ sdk.DeploymentPlugin[config.AnsiblePluginConfig, config.AnsibleDeployTargetConfig, config.AnsibleApplicationSpec] = (*Plugin)(nil)
var _ sdk.LivestatePlugin[config.AnsiblePluginConfig, config.AnsibleDeployTargetConfig, config.AnsibleApplicationSpec] = (*Plugin)(nil)

const (
	AnsibleSync Stage = "ANSIBLE_SYNC"
	// TODO: Add rollback stage
	AnsibleRollback Stage = "ANSIBLE_ROLLBACK"
)

type Stage string

var allStages = []string{
	string(AnsibleSync),
}

func (p *Plugin) FetchDefinedStages() []string {
	return allStages
}

func (p *Plugin) BuildPipelineSyncStages(ctx context.Context, cfg *config.AnsiblePluginConfig, input *sdk.BuildPipelineSyncStagesInput) (*sdk.BuildPipelineSyncStagesResponse, error) {
	stages, err := buildPipeline(input.Request.Stages, input.Request.Rollback, input.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to build pipeline stages: %w", err)
	}
	return &sdk.BuildPipelineSyncStagesResponse{
		Stages: stages,
	}, nil
}

func (p *Plugin) ExecuteStage(ctx context.Context, cfg *config.AnsiblePluginConfig, dts []*sdk.DeployTarget[config.AnsibleDeployTargetConfig], input *sdk.ExecuteStageInput[config.AnsibleApplicationSpec]) (*sdk.ExecuteStageResponse, error) {
	switch input.Request.StageName {
	case string(AnsibleSync):
		return &sdk.ExecuteStageResponse{
			Status: p.executeAnsibleSyncStage(ctx, cfg, dts, input),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported stage: %s", input.Request.StageName)
	}
}

func (p *Plugin) DetermineVersions(ctx context.Context, cfg *config.AnsiblePluginConfig, d *sdk.DetermineVersionsInput[config.AnsibleApplicationSpec]) (*sdk.DetermineVersionsResponse, error) {
	appCfg, err := d.Request.DeploymentSource.AppConfig()
	if err != nil {
		return nil, err
	}

	return &sdk.DetermineVersionsResponse{
		Versions: []sdk.ArtifactVersion{
			{
				Name:    "playbook",
				Version: appCfg.Spec.Playbook.Path,
				URL:     appCfg.Spec.Playbook.Path,
			},
		},
	}, nil
}

func (p *Plugin) DetermineStrategy(ctx context.Context, cfg *config.AnsiblePluginConfig, d *sdk.DetermineStrategyInput[config.AnsibleApplicationSpec]) (*sdk.DetermineStrategyResponse, error) {
	appCfg, err := d.Request.TargetDeploymentSource.AppConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get app config: %v", err)
	}

	// Always use pipeline sync strategy for applications with any stages defined
	// This ensures that the UI shows pipeline stages for better visibility
	// Check if pipeline stages are defined in the application configuration
	if appCfg.HasStage(string(AnsibleSync)) {
		// Use pipeline strategy when ANSIBLE_SYNC stage is explicitly defined
		return &sdk.DetermineStrategyResponse{
			Strategy: sdk.SyncStrategyPipelineSync,
			Summary:  "Using PipelineSync because ANSIBLE_SYNC stage is defined",
		}, nil
	}

	// For Ansible plugin, if no specific stages are defined, default to pipeline sync
	// with a single ANSIBLE_SYNC stage to ensure stages are visible in UI
	return &sdk.DetermineStrategyResponse{
		Strategy: sdk.SyncStrategyPipelineSync,
		Summary:  "Using PipelineSync as default for Ansible deployments to show stages in UI",
	}, nil
}

func (p *Plugin) BuildQuickSyncStages(ctx context.Context, cfg *config.AnsiblePluginConfig, input *sdk.BuildQuickSyncStagesInput) (*sdk.BuildQuickSyncStagesResponse, error) {
	return &sdk.BuildQuickSyncStagesResponse{
		Stages: buildQuickSync(input.Request.Rollback),
	}, nil
}

func buildQuickSync(autoRollback bool) []sdk.QuickSyncStage {
	out := make([]sdk.QuickSyncStage, 0, 1)
	out = append(out, sdk.QuickSyncStage{
		Name:        string(AnsibleSync),
		Description: "Execute Ansible playbook",
		Metadata: map[string]string{
			sdk.MetadataKeyStageDisplay: "Deploy Ansible Configuration",
		},
		AvailableOperation: sdk.ManualOperationNone,
	})
	// Note: rollback is not supported for Ansible plugin
	return out
}

func initialMetadata(s sdk.StageConfig, logger *zap.Logger) (map[string]string, error) {
	switch s.Name {
	case string(AnsibleSync):
		return map[string]string{
			sdk.MetadataKeyStageDisplay: "Deploy Ansible Configuration",
		}, nil
	default:
		return map[string]string{
			sdk.MetadataKeyStageDisplay: s.Name,
		}, nil
	}
}

func buildPipeline(stages []sdk.StageConfig, autoRollback bool, logger *zap.Logger) ([]sdk.PipelineStage, error) {
	out := make([]sdk.PipelineStage, 0, len(stages))

	// If no stages are provided, create a default ANSIBLE_SYNC stage
	if len(stages) == 0 {
		metadata, err := initialMetadata(sdk.StageConfig{Name: string(AnsibleSync)}, logger)
		if err != nil {
			return nil, err
		}
		out = append(out, sdk.PipelineStage{
			Name:               string(AnsibleSync),
			Index:              0,
			Rollback:           false,
			Metadata:           metadata,
			AvailableOperation: sdk.ManualOperationNone,
		})
	} else {
		// Use provided stages
		for _, s := range stages {
			metadata, err := initialMetadata(s, logger)
			if err != nil {
				return nil, err
			}
			out = append(out, sdk.PipelineStage{
				Name:               s.Name,
				Index:              s.Index,
				Rollback:           false,
				Metadata:           metadata,
				AvailableOperation: sdk.ManualOperationNone,
			})
		}
	}

	// Note: rollback is not supported for Ansible plugin
	return out, nil
}

func (p *Plugin) executeAnsibleSyncStage(ctx context.Context, cfg *config.AnsiblePluginConfig, dts []*sdk.DeployTarget[config.AnsibleDeployTargetConfig], input *sdk.ExecuteStageInput[config.AnsibleApplicationSpec]) sdk.StageStatus {
	lp := input.Client.LogPersister()

	appCfg, err := input.Request.TargetDeploymentSource.AppConfig()
	if err != nil {
		lp.Errorf("Failed to get app config: %v", err)
		return sdk.StageStatusFailure
	}

	// Handle multiple deploy targets
	if len(dts) == 0 {
		lp.Infof("🎯 No deploy targets specified, using default configuration")
		dtConfig := &config.AnsibleDeployTargetConfig{}
		return p.executeAnsiblePlaybook(ctx, cfg, dtConfig, &appCfg.Spec.Playbook, input)
	}

	lp.Infof("🎯 Executing deployment across %d deploy target(s)", len(dts))

	// Execute playbook for each deploy target
	for i, dt := range dts {
		lp.Infof("📋 Deploy Target %d/%d: %s", i+1, len(dts), dt.Name)

		// Log deploy target labels for debugging
		if len(dt.Labels) > 0 {
			lp.Infof("🏷️  Target labels: %v", dt.Labels)
		}

		status := p.executeAnsiblePlaybook(ctx, cfg, &dt.Config, &appCfg.Spec.Playbook, input)

		if status != sdk.StageStatusSuccess {
			lp.Errorf("❌ Failed to execute playbook for deploy target: %s", dt.Name)
			return status
		}

		lp.Infof("✅ Successfully executed playbook for deploy target: %s", dt.Name)
	}

	lp.Infof("🎉 All deploy targets completed successfully!")
	return sdk.StageStatusSuccess
}

func (p *Plugin) executeAnsiblePlaybook(ctx context.Context, cfg *config.AnsiblePluginConfig, dtConfig *config.AnsibleDeployTargetConfig, playbookConfig *config.AnsiblePlaybookManifest, input *sdk.ExecuteStageInput[config.AnsibleApplicationSpec]) sdk.StageStatus {
	lp := input.Client.LogPersister()

	// Log deployment start
	lp.Infof("🚀 Starting Ansible deployment")
	lp.Infof("📁 Playbook: %s", playbookConfig.Path)
	lp.Infof("🎯 Target directory: %s", input.Request.TargetDeploymentSource.ApplicationDirectory)

	appDir := input.Request.TargetDeploymentSource.ApplicationDirectory
	playbookPath := filepath.Join(appDir, playbookConfig.Path)

	if _, err := os.Stat(playbookPath); os.IsNotExist(err) {
		lp.Errorf("❌ Playbook file does not exist: %s", playbookPath)

		// Debug: List directory contents to help diagnose
		if dirContent, err := os.ReadDir(appDir); err == nil {
			lp.Infof("📂 ApplicationDirectory contents:")
			for _, entry := range dirContent {
				lp.Infof("  - %s (isDir: %v)", entry.Name(), entry.IsDir())
			}
		}

		return sdk.StageStatusFailure
	}

	lp.Infof("✅ Playbook file found: %s", playbookPath)

	// Build command arguments using helper functions
	lp.Infof("🔧 Building ansible-playbook arguments...")
	ansiblePath, args := p.buildAnsibleCommand(dtConfig, playbookConfig, appDir, false, false, lp)

	lp.Infof("🚀 Executing ansible-playbook command: %s %s", ansiblePath, strings.Join(args, " "))

	// Determine working directory
	workingDir := input.Request.TargetDeploymentSource.ApplicationDirectory
	if dtConfig.WorkingDirectory != "" {
		if filepath.IsAbs(dtConfig.WorkingDirectory) {
			workingDir = dtConfig.WorkingDirectory
		} else {
			workingDir = filepath.Join(input.Request.TargetDeploymentSource.ApplicationDirectory, dtConfig.WorkingDirectory)
		}
	}
	lp.Infof("📁 Working directory: %s", workingDir)

	// Track execution timing
	startTime := time.Now()
	lp.Infof("⏱️  Execution started at: %s", startTime.Format("2006-01-02 15:04:05"))

	// Apply timeout - use deploy target command timeout if available, otherwise playbook timeout
	cmdCtx := ctx
	timeout := playbookConfig.Timeout
	if dtConfig.CommandTimeout > 0 {
		timeout = dtConfig.CommandTimeout
		lp.Infof("⏰ Using deploy target command timeout: %d seconds", timeout)
	} else if playbookConfig.Timeout > 0 {
		lp.Infof("⏰ Using playbook timeout: %d seconds", timeout)
	}

	if timeout > 0 {
		var cancel context.CancelFunc
		cmdCtx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}

	cmd := exec.CommandContext(cmdCtx, ansiblePath, args...)
	cmd.Dir = workingDir

	// Set up environment variables using helper function
	env := p.setupEnvironment(dtConfig)
	if len(dtConfig.Env) > 0 {
		lp.Infof("🌍 Setting environment variables from deploy target:")
		for key, value := range dtConfig.Env {
			lp.Infof("  %s=%s", key, value)
		}
	}
	cmd.Env = env
	cmd.Stdout = lp
	cmd.Stderr = lp

	lp.Infof("📜 Ansible playbook output:")
	lp.Infof("=" + strings.Repeat("=", 80))

	if err := cmd.Run(); err != nil {
		duration := time.Since(startTime)
		lp.Errorf("❌ Failed to execute ansible-playbook: %v", err)
		lp.Errorf("💥 Exit code: %v", cmd.ProcessState.ExitCode())
		lp.Errorf("⏱️  Execution duration: %v", duration)
		lp.Errorf("=" + strings.Repeat("=", 80))
		return sdk.StageStatusFailure
	}

	duration := time.Since(startTime)
	lp.Infof("=" + strings.Repeat("=", 80))
	lp.Infof("✅ Ansible playbook executed successfully!")
	lp.Infof("⏱️  Total execution time: %v", duration)
	lp.Infof("🎉 Deployment completed at: %s", time.Now().Format("2006-01-02 15:04:05"))
	return sdk.StageStatusSuccess
}

// GetLivestate implements sdk.LivestatePlugin for Ansible.
// It returns the live state of ansible resources and sync status.
func (p *Plugin) GetLivestate(ctx context.Context, cfg *config.AnsiblePluginConfig, dts []*sdk.DeployTarget[config.AnsibleDeployTargetConfig], input *sdk.GetLivestateInput[config.AnsibleApplicationSpec]) (*sdk.GetLivestateResponse, error) {
	appCfg, err := input.Request.DeploymentSource.AppConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get app config: %v", err)
	}

	// Build live state by checking ansible resources
	liveState, err := p.getAnsibleLiveState(ctx, cfg, dts, &appCfg.Spec.Playbook, input)
	if err != nil {
		input.Logger.Error("Failed to get ansible live state", zap.Error(err))
		// Return partial response with error info
		return &sdk.GetLivestateResponse{
			LiveState: sdk.ApplicationLiveState{
				Resources: []sdk.ResourceState{
					{
						ID:                "ansible-playbook-error",
						Name:              "Ansible Playbook",
						ResourceType:      "ansible-playbook",
						HealthStatus:      sdk.ResourceHealthStateUnknown,
						HealthDescription: fmt.Sprintf("Error getting live state: %v", err),
						DeployTarget:      getDeployTargetName(dts),
						CreatedAt:         time.Now(),
					},
				},
			},
			SyncState: sdk.ApplicationSyncState{
				Status:      sdk.ApplicationSyncStateUnknown,
				ShortReason: "Unable to determine sync state due to live state error",
				Reason:      fmt.Sprintf("Failed to retrieve live state: %v", err),
			},
		}, nil
	}

	// Build sync state by comparing desired vs actual state
	syncState, err := p.getAnsibleSyncState(ctx, cfg, dts, &appCfg.Spec.Playbook, input)
	if err != nil {
		input.Logger.Error("Failed to get ansible sync state", zap.Error(err))
		// Use default sync state if sync check fails
		syncState = &sdk.ApplicationSyncState{
			Status:      sdk.ApplicationSyncStateUnknown,
			ShortReason: "Unable to determine sync state",
			Reason:      fmt.Sprintf("Sync state check failed: %v", err),
		}
	}

	return &sdk.GetLivestateResponse{
		LiveState: *liveState,
		SyncState: *syncState,
	}, nil
}

// getAnsibleLiveState checks the current state of ansible resources
func (p *Plugin) getAnsibleLiveState(ctx context.Context, cfg *config.AnsiblePluginConfig, dts []*sdk.DeployTarget[config.AnsibleDeployTargetConfig], playbookConfig *config.AnsiblePlaybookManifest, input *sdk.GetLivestateInput[config.AnsibleApplicationSpec]) (*sdk.ApplicationLiveState, error) {
	resources := make([]sdk.ResourceState, 0)

	// Handle multiple deploy targets or default configuration
	if len(dts) == 0 {
		// Use default configuration
		dtConfig := &config.AnsibleDeployTargetConfig{}
		resource, err := p.checkAnsibleResource(ctx, cfg, dtConfig, playbookConfig, input, "default")
		if err != nil {
			return nil, fmt.Errorf("failed to check default ansible resource: %v", err)
		}
		resources = append(resources, *resource)
	} else {
		// Check each deploy target
		for _, dt := range dts {
			resource, err := p.checkAnsibleResource(ctx, cfg, &dt.Config, playbookConfig, input, dt.Name)
			if err != nil {
				return nil, fmt.Errorf("failed to check ansible resource for deploy target %s: %v", dt.Name, err)
			}
			resources = append(resources, *resource)
		}
	}

	return &sdk.ApplicationLiveState{
		Resources: resources,
	}, nil
}

// checkAnsibleResource checks the state of an ansible resource for a deploy target
func (p *Plugin) checkAnsibleResource(ctx context.Context, cfg *config.AnsiblePluginConfig, dtConfig *config.AnsibleDeployTargetConfig, playbookConfig *config.AnsiblePlaybookManifest, input *sdk.GetLivestateInput[config.AnsibleApplicationSpec], deployTargetName string) (*sdk.ResourceState, error) {
	// Check if playbook file exists
	playbookPath := filepath.Join(input.Request.DeploymentSource.ApplicationDirectory, playbookConfig.Path)
	resourceID := fmt.Sprintf("ansible-playbook-%s-%s", deployTargetName, playbookConfig.Path)

	resourceState := &sdk.ResourceState{
		ID:           resourceID,
		Name:         fmt.Sprintf("Ansible Playbook (%s)", playbookConfig.Path),
		ResourceType: "ansible-playbook",
		DeployTarget: deployTargetName,
		CreatedAt:    time.Now(),
		ResourceMetadata: map[string]string{
			"playbook_path": playbookConfig.Path,
			"deploy_target": deployTargetName,
			"verbosity":     fmt.Sprintf("%d", playbookConfig.Verbosity),
			"check_mode":    fmt.Sprintf("%t", playbookConfig.CheckMode),
		},
	}

	// Add inventory info if present
	inventory := playbookConfig.Inventory
	if inventory == "" {
		inventory = dtConfig.Inventory
	}
	if inventory != "" {
		resourceState.ResourceMetadata["inventory"] = inventory
	}

	// Check playbook file exists
	if _, err := os.Stat(playbookPath); os.IsNotExist(err) {
		resourceState.HealthStatus = sdk.ResourceHealthStateUnhealthy
		resourceState.HealthDescription = fmt.Sprintf("Playbook file not found: %s", playbookPath)
		return resourceState, nil
	}

	// Check ansible executable availability
	ansiblePath := dtConfig.AnsiblePath
	if ansiblePath == "" {
		ansiblePath = "ansible-playbook"
	}

	// Test ansible-playbook availability with --version
	cmd := exec.CommandContext(ctx, ansiblePath, "--version")
	if err := cmd.Run(); err != nil {
		resourceState.HealthStatus = sdk.ResourceHealthStateUnhealthy
		resourceState.HealthDescription = fmt.Sprintf("Ansible executable not available: %s", ansiblePath)
		return resourceState, nil
	}

	// If inventory is specified, check connectivity to hosts
	if inventory != "" {
		inventoryPath := filepath.Join(input.Request.DeploymentSource.ApplicationDirectory, inventory)
		if _, err := os.Stat(inventoryPath); os.IsNotExist(err) {
			resourceState.HealthStatus = sdk.ResourceHealthStateUnhealthy
			resourceState.HealthDescription = fmt.Sprintf("Inventory file not found: %s", inventoryPath)
			return resourceState, nil
		}

		// Test connectivity with ansible ping (with timeout)
		connectivityCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		pingCmd := exec.CommandContext(connectivityCtx, "ansible", "all", "-i", inventoryPath, "-m", "ping", "--one-line")

		// Add SSH settings if present
		if dtConfig.SSH != nil {
			if dtConfig.SSH.User != "" {
				pingCmd.Args = append(pingCmd.Args, "--user", dtConfig.SSH.User)
			}
			if dtConfig.SSH.PrivateKeyFile != "" {
				keyPath := filepath.Join(input.Request.DeploymentSource.ApplicationDirectory, dtConfig.SSH.PrivateKeyFile)
				pingCmd.Args = append(pingCmd.Args, "--private-key", keyPath)
			}
		}

		// Set environment variables
		env := os.Environ()
		if len(dtConfig.Env) > 0 {
			for key, value := range dtConfig.Env {
				env = append(env, fmt.Sprintf("%s=%s", key, value))
			}
		}
		pingCmd.Env = env

		output, err := pingCmd.Output()
		if err != nil {
			resourceState.HealthStatus = sdk.ResourceHealthStateUnhealthy
			resourceState.HealthDescription = fmt.Sprintf("Host connectivity check failed: %v", err)
			// Add connectivity test output as metadata if available
			if len(output) > 0 {
				resourceState.ResourceMetadata["connectivity_output"] = string(output)
			}
			return resourceState, nil
		}

		// Parse ping output for detailed host status
		resourceState.ResourceMetadata["connectivity_output"] = string(output)
	}

	// If all checks pass, mark as healthy
	resourceState.HealthStatus = sdk.ResourceHealthStateHealthy
	resourceState.HealthDescription = "Playbook and connectivity checks passed"

	return resourceState, nil
}

// getAnsibleSyncState determines if the ansible configuration is in sync
func (p *Plugin) getAnsibleSyncState(ctx context.Context, cfg *config.AnsiblePluginConfig, dts []*sdk.DeployTarget[config.AnsibleDeployTargetConfig], playbookConfig *config.AnsiblePlaybookManifest, input *sdk.GetLivestateInput[config.AnsibleApplicationSpec]) (*sdk.ApplicationSyncState, error) {
	// For ansible, sync state is determined by running playbook in check mode
	// and comparing the output to see if any changes would be made

	// Use first deploy target for sync check (or default config)
	var dtConfig *config.AnsibleDeployTargetConfig
	deployTargetName := "default"
	if len(dts) > 0 {
		dtConfig = &dts[0].Config
		deployTargetName = dts[0].Name
	} else {
		dtConfig = &config.AnsibleDeployTargetConfig{}
	}

	// Run ansible-playbook in check mode to detect changes
	changed, reason, err := p.runAnsibleCheckMode(ctx, cfg, dtConfig, playbookConfig, input)
	if err != nil {
		return &sdk.ApplicationSyncState{
			Status:      sdk.ApplicationSyncStateUnknown,
			ShortReason: "Failed to run sync check",
			Reason:      fmt.Sprintf("Error running ansible-playbook --check: %v", err),
		}, nil
	}

	if changed {
		commit := input.Request.DeploymentSource.CommitHash
		if len(commit) > 7 {
			commit = commit[:7]
		}
		return &sdk.ApplicationSyncState{
			Status:      sdk.ApplicationSyncStateOutOfSync,
			ShortReason: fmt.Sprintf("Configuration drift detected on %s", deployTargetName),
			Reason:      fmt.Sprintf("Diff between the defined state in Git at commit %s and actual live state:\n\n%s", commit, reason),
		}, nil
	}

	return &sdk.ApplicationSyncState{
		Status:      sdk.ApplicationSyncStateSynced,
		ShortReason: "No configuration changes required",
		Reason:      "Ansible playbook check mode indicates no changes needed",
	}, nil
}

// runAnsibleCheckMode runs ansible-playbook in check mode to detect configuration drift
func (p *Plugin) runAnsibleCheckMode(ctx context.Context, cfg *config.AnsiblePluginConfig, dtConfig *config.AnsibleDeployTargetConfig, playbookConfig *config.AnsiblePlaybookManifest, input *sdk.GetLivestateInput[config.AnsibleApplicationSpec]) (bool, string, error) {
	// Create a dummy log persister for check mode (since we don't have access to stage log persister here)
	// We'll use a simple logger that doesn't persist logs
	dummyLogger := &dummyLogPersister{}

	appDir := input.Request.DeploymentSource.ApplicationDirectory

	// Build command arguments using helper functions with forced check and diff modes
	ansiblePath, args := p.buildAnsibleCommand(dtConfig, playbookConfig, appDir, true, true, dummyLogger)

	// Apply timeout - shorter timeout for check mode
	checkCtx := ctx
	timeout := 60 // 1 minute default for check mode
	if dtConfig.CommandTimeout > 0 && dtConfig.CommandTimeout < 300 {
		timeout = dtConfig.CommandTimeout
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		checkCtx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}

	cmd := exec.CommandContext(checkCtx, ansiblePath, args...)
	cmd.Dir = appDir

	// Set up environment variables using helper function
	cmd.Env = p.setupEnvironment(dtConfig)

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// ansible-playbook --check returns exit code 2 when changes are detected
	// exit code 0 means no changes, other codes indicate errors
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 2 {
				// Changes detected - parse output for change details
				return true, outputStr, nil
			}
		}
		// Real error occurred
		return false, "", fmt.Errorf("ansible check mode failed: %v, output: %s", err, outputStr)
	}

	// No changes needed
	return false, outputStr, nil
}

// getDeployTargetName returns deploy target name for resource identification
func getDeployTargetName(dts []*sdk.DeployTarget[config.AnsibleDeployTargetConfig]) string {
	if len(dts) > 0 {
		return dts[0].Name
	}
	return "default"
}

// Helper functions for building ansible command arguments

// buildAnsibleExecutable returns the ansible-playbook executable path
func (p *Plugin) buildAnsibleExecutable(dtConfig *config.AnsibleDeployTargetConfig, lp sdk.StageLogPersister) string {
	ansiblePath := dtConfig.AnsiblePath
	if ansiblePath == "" {
		ansiblePath = "ansible-playbook"
	}
	lp.Infof("⚙️  Ansible executable: %s", ansiblePath)
	return ansiblePath
}

// addInventoryArgs adds inventory arguments to the command
func (p *Plugin) addInventoryArgs(args []string, dtConfig *config.AnsibleDeployTargetConfig, playbookConfig *config.AnsiblePlaybookManifest, appDir string, lp sdk.StageLogPersister) []string {
	inventory := playbookConfig.Inventory
	if inventory == "" {
		inventory = dtConfig.Inventory
	}
	if inventory != "" {
		inventoryPath := filepath.Join(appDir, inventory)
		args = append(args, "-i", inventoryPath)
		lp.Infof("📋 Using inventory: %s", inventoryPath)
	} else {
		lp.Infof("📋 No inventory specified, using default")
	}
	return args
}

// addVaultArgs adds vault password file arguments to the command
func (p *Plugin) addVaultArgs(args []string, dtConfig *config.AnsibleDeployTargetConfig, playbookConfig *config.AnsiblePlaybookManifest, appDir string, lp sdk.StageLogPersister) []string {
	vault := playbookConfig.Vault
	if vault == "" {
		vault = dtConfig.Vault
	}
	if vault != "" {
		vaultPath := filepath.Join(appDir, vault)
		args = append(args, "--vault-password-file", vaultPath)
		lp.Infof("🔐 Using vault password file: %s", vaultPath)
	}
	return args
}

// addExtraVarsArgs adds extra variables arguments to the command
func (p *Plugin) addExtraVarsArgs(args []string, playbookConfig *config.AnsiblePlaybookManifest, lp sdk.StageLogPersister) []string {
	if len(playbookConfig.ExtraVars) > 0 {
		extraVars := make([]string, 0, len(playbookConfig.ExtraVars))
		for k, v := range playbookConfig.ExtraVars {
			extraVars = append(extraVars, fmt.Sprintf("%s=%s", k, v))
		}
		args = append(args, "--extra-vars", strings.Join(extraVars, " "))
		lp.Infof("🔧 Extra variables: %v", playbookConfig.ExtraVars)
	}
	return args
}

// addTagsArgs adds tags and skip-tags arguments to the command
func (p *Plugin) addTagsArgs(args []string, playbookConfig *config.AnsiblePlaybookManifest, lp sdk.StageLogPersister) []string {
	// Tags
	if len(playbookConfig.Tags) > 0 {
		args = append(args, "--tags", strings.Join(playbookConfig.Tags, ","))
		lp.Infof("🏷️  Running with tags: %v", playbookConfig.Tags)
	}

	// Skip tags
	if len(playbookConfig.SkipTags) > 0 {
		args = append(args, "--skip-tags", strings.Join(playbookConfig.SkipTags, ","))
		lp.Infof("🚫 Skipping tags: %v", playbookConfig.SkipTags)
	}

	return args
}

// addLimitArgs adds limit arguments to the command
func (p *Plugin) addLimitArgs(args []string, playbookConfig *config.AnsiblePlaybookManifest, lp sdk.StageLogPersister) []string {
	if playbookConfig.Limit != "" {
		args = append(args, "--limit", playbookConfig.Limit)
		lp.Infof("🎯 Limiting to hosts: %s", playbookConfig.Limit)
	}
	return args
}

// addVerbosityArgs adds verbosity arguments to the command
func (p *Plugin) addVerbosityArgs(args []string, playbookConfig *config.AnsiblePlaybookManifest, lp sdk.StageLogPersister) []string {
	if playbookConfig.Verbosity > 0 {
		verbosity := strings.Repeat("v", playbookConfig.Verbosity)
		args = append(args, fmt.Sprintf("-%s", verbosity))
		lp.Infof("📢 Verbosity level: %d", playbookConfig.Verbosity)
	}
	return args
}

// addModeArgs adds check and diff mode arguments to the command
func (p *Plugin) addModeArgs(args []string, playbookConfig *config.AnsiblePlaybookManifest, forceCheck bool, forceDiff bool, lp sdk.StageLogPersister) []string {
	// Check mode
	if playbookConfig.CheckMode || forceCheck {
		args = append(args, "--check")
		lp.Infof("🔍 Running in check mode (dry-run)")
	}

	// Diff mode
	if playbookConfig.DiffMode || forceDiff {
		args = append(args, "--diff")
		lp.Infof("📊 Diff mode enabled")
	}

	return args
}

// addSSHArgs adds SSH-related arguments to the command
func (p *Plugin) addSSHArgs(args []string, dtConfig *config.AnsibleDeployTargetConfig, playbookConfig *config.AnsiblePlaybookManifest, appDir string, lp sdk.StageLogPersister) []string {
	// Private key from playbook config (takes precedence)
	if playbookConfig.PrivateKey != "" {
		keyPath := filepath.Join(appDir, playbookConfig.PrivateKey)
		args = append(args, "--private-key", keyPath)
		lp.Infof("🔑 Using private key: %s", keyPath)
	}

	// Remote user from playbook config
	if playbookConfig.RemoteUser != "" {
		args = append(args, "--user", playbookConfig.RemoteUser)
		lp.Infof("👤 Remote user: %s", playbookConfig.RemoteUser)
	}

	// Become user from playbook config
	if playbookConfig.BecomeUser != "" {
		args = append(args, "--become", "--become-user", playbookConfig.BecomeUser)
		lp.Infof("🔓 Become user: %s", playbookConfig.BecomeUser)
	}

	// SSH settings from deploy target (fallback/additional)
	if dtConfig.SSH != nil {
		if dtConfig.SSH.User != "" && playbookConfig.RemoteUser == "" {
			args = append(args, "--user", dtConfig.SSH.User)
			lp.Infof("👤 SSH user (from deploy target): %s", dtConfig.SSH.User)
		}
		if dtConfig.SSH.PrivateKeyFile != "" && playbookConfig.PrivateKey == "" {
			keyPath := filepath.Join(appDir, dtConfig.SSH.PrivateKeyFile)
			args = append(args, "--private-key", keyPath)
			lp.Infof("🔑 SSH private key (from deploy target): %s", keyPath)
		}
		if dtConfig.SSH.Port > 0 {
			args = append(args, "--extra-vars", fmt.Sprintf("ansible_ssh_port=%d", dtConfig.SSH.Port))
			lp.Infof("🔌 SSH port (from deploy target): %d", dtConfig.SSH.Port)
		}
	}

	return args
}

// addDeployTargetArgs adds deploy target specific arguments to the command
func (p *Plugin) addDeployTargetArgs(args []string, dtConfig *config.AnsibleDeployTargetConfig, lp sdk.StageLogPersister) []string {
	// Connection timeout from deploy target
	if dtConfig.ConnectionTimeout > 0 {
		args = append(args, "--extra-vars", fmt.Sprintf("ansible_ssh_timeout=%d", dtConfig.ConnectionTimeout))
		lp.Infof("⏰ SSH connection timeout: %d seconds", dtConfig.ConnectionTimeout)
	}

	// Host key checking
	if dtConfig.HostKeyChecking != nil {
		hostKeyCheck := "True"
		if !*dtConfig.HostKeyChecking {
			hostKeyCheck = "False"
		}
		args = append(args, "--extra-vars", fmt.Sprintf("ansible_ssh_host_key_checking=%s", hostKeyCheck))
		lp.Infof("🔒 SSH host key checking: %s", hostKeyCheck)
	}

	// Extra flags from deploy target
	if len(dtConfig.ExtraFlags) > 0 {
		args = append(args, dtConfig.ExtraFlags...)
		lp.Infof("🔧 Extra flags (from deploy target): %v", dtConfig.ExtraFlags)
	}

	return args
}

// setupEnvironment sets up environment variables for the command
func (p *Plugin) setupEnvironment(dtConfig *config.AnsibleDeployTargetConfig) []string {
	env := os.Environ()
	if len(dtConfig.Env) > 0 {
		for key, value := range dtConfig.Env {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
	}
	return env
}

// buildAnsibleCommand builds the complete ansible-playbook command with all arguments
func (p *Plugin) buildAnsibleCommand(
	dtConfig *config.AnsibleDeployTargetConfig,
	playbookConfig *config.AnsiblePlaybookManifest,
	appDir string,
	forceCheck bool,
	forceDiff bool,
	lp sdk.StageLogPersister,
) (string, []string) {
	// Get ansible executable
	ansiblePath := p.buildAnsibleExecutable(dtConfig, lp)

	// Build playbook path
	playbookPath := filepath.Join(appDir, playbookConfig.Path)
	args := []string{playbookPath}

	// Add all argument types
	args = p.addModeArgs(args, playbookConfig, forceCheck, forceDiff, lp)
	args = p.addInventoryArgs(args, dtConfig, playbookConfig, appDir, lp)
	args = p.addVaultArgs(args, dtConfig, playbookConfig, appDir, lp)
	args = p.addExtraVarsArgs(args, playbookConfig, lp)
	args = p.addTagsArgs(args, playbookConfig, lp)
	args = p.addLimitArgs(args, playbookConfig, lp)
	args = p.addVerbosityArgs(args, playbookConfig, lp)
	args = p.addSSHArgs(args, dtConfig, playbookConfig, appDir, lp)
	args = p.addDeployTargetArgs(args, dtConfig, lp)

	return ansiblePath, args
}

// dummyLogPersister is a simple log persister that doesn't actually persist logs
// Used for check mode where we don't have access to stage log persister
type dummyLogPersister struct{}

func (d *dummyLogPersister) Write(log []byte) (int, error)            { return len(log), nil }
func (d *dummyLogPersister) Info(log string)                          {}
func (d *dummyLogPersister) Infof(format string, a ...interface{})    {}
func (d *dummyLogPersister) Success(log string)                       {}
func (d *dummyLogPersister) Successf(format string, a ...interface{}) {}
func (d *dummyLogPersister) Error(log string)                         {}
func (d *dummyLogPersister) Errorf(format string, a ...interface{})   {}
