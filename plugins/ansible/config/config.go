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

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AnsiblePluginConfig represents the plugin-level configuration for Ansible.
type AnsiblePluginConfig struct{}

// Validate validates the plugin configuration.
func (c *AnsiblePluginConfig) Validate() error {
	// No validation needed for empty plugin config
	return nil
}

// AnsibleDeployTargetConfig represents the configuration for Ansible deployment targets.
type AnsibleDeployTargetConfig struct {
	// AnsiblePath specifies the path to the ansible-playbook executable
	AnsiblePath string `json:"ansiblePath,omitempty"`
	
	// Inventory specifies the default inventory file path
	Inventory string `json:"inventory,omitempty"`
	
	// Vault specifies the default vault password file path
	Vault string `json:"vault,omitempty"`
	
	// WorkingDirectory specifies the working directory for ansible commands
	WorkingDirectory string `json:"workingDirectory,omitempty"`
	
	// Environment variables to set for ansible commands
	Env map[string]string `json:"env,omitempty"`
	
	// SSH connection settings
	SSH *AnsibleSSHConfig `json:"ssh,omitempty"`
	
	// Connection timeout in seconds
	ConnectionTimeout int `json:"connectionTimeout,omitempty"`
	
	// Command timeout in seconds (default timeout for all ansible commands)
	CommandTimeout int `json:"commandTimeout,omitempty"`
	
	// HostKeyChecking controls SSH host key checking (default: true)
	HostKeyChecking *bool `json:"hostKeyChecking,omitempty"`
	
	// Additional command line flags to pass to ansible-playbook
	ExtraFlags []string `json:"extraFlags,omitempty"`
}

// AnsibleSSHConfig represents SSH connection configuration.
type AnsibleSSHConfig struct {
	// PrivateKeyFile specifies the SSH private key file path
	PrivateKeyFile string `json:"privateKeyFile,omitempty"`
	
	// User specifies the SSH user
	User string `json:"user,omitempty"`
	
	// Port specifies the SSH port
	Port int `json:"port,omitempty"`
	
	// ConnectionAttempts specifies the number of SSH connection attempts
	ConnectionAttempts int `json:"connectionAttempts,omitempty"`
	
	// ControlPersist enables SSH connection multiplexing
	ControlPersist string `json:"controlPersist,omitempty"`
}

// Validate validates the deploy target configuration.
func (c *AnsibleDeployTargetConfig) Validate() error {
	// Validate ansible path if specified
	if c.AnsiblePath != "" {
		if !filepath.IsAbs(c.AnsiblePath) && !strings.Contains(c.AnsiblePath, "/") {
			// Allow relative executable names like "ansible-playbook"
			// But validate absolute paths
		} else if filepath.IsAbs(c.AnsiblePath) {
			if _, err := os.Stat(c.AnsiblePath); os.IsNotExist(err) {
				return fmt.Errorf("ansible executable not found at path: %s", c.AnsiblePath)
			}
		}
	}

	// Validate working directory if specified
	if c.WorkingDirectory != "" && filepath.IsAbs(c.WorkingDirectory) {
		if _, err := os.Stat(c.WorkingDirectory); os.IsNotExist(err) {
			return fmt.Errorf("working directory not found: %s", c.WorkingDirectory)
		}
	}

	// Validate timeouts
	if c.ConnectionTimeout < 0 {
		return fmt.Errorf("connection timeout must be non-negative, got: %d", c.ConnectionTimeout)
	}
	if c.CommandTimeout < 0 {
		return fmt.Errorf("command timeout must be non-negative, got: %d", c.CommandTimeout)
	}

	// Validate SSH configuration if specified
	if c.SSH != nil {
		if err := c.SSH.Validate(); err != nil {
			return fmt.Errorf("invalid SSH configuration: %w", err)
		}
	}

	// Validate environment variables don't have empty keys
	for key := range c.Env {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("environment variable key cannot be empty")
		}
	}

	// Note: inventory and vault files are validated relative to application directory at runtime
	return nil
}

// Validate validates the SSH configuration.
func (c *AnsibleSSHConfig) Validate() error {
	// Validate SSH port
	if c.Port < 0 || c.Port > 65535 {
		return fmt.Errorf("SSH port must be between 0 and 65535, got: %d", c.Port)
	}

	// Validate connection attempts
	if c.ConnectionAttempts < 0 {
		return fmt.Errorf("connection attempts must be non-negative, got: %d", c.ConnectionAttempts)
	}

	// Validate private key file if specified (absolute path validation)
	if c.PrivateKeyFile != "" && filepath.IsAbs(c.PrivateKeyFile) {
		if _, err := os.Stat(c.PrivateKeyFile); os.IsNotExist(err) {
			return fmt.Errorf("SSH private key file not found: %s", c.PrivateKeyFile)
		}
	}

	return nil
}

// AnsibleApplicationSpec represents the configuration for Ansible applications.
type AnsibleApplicationSpec struct {
	Name     string                  `json:"name,omitempty"`
	Playbook AnsiblePlaybookManifest `json:"playbook"`
	Pipeline *AnsiblePipelineSpec    `json:"pipeline,omitempty"`
}

// Validate validates the application specification.
func (c *AnsibleApplicationSpec) Validate() error {
	// Validate playbook configuration
	if err := c.Playbook.Validate(); err != nil {
		return fmt.Errorf("invalid playbook configuration: %w", err)
	}

	// Validate pipeline if specified
	if c.Pipeline != nil {
		if err := c.Pipeline.Validate(); err != nil {
			return fmt.Errorf("invalid pipeline configuration: %w", err)
		}
	}

	return nil
}

// AnsiblePlaybookManifest represents the Ansible playbook configuration.
type AnsiblePlaybookManifest struct {
	Path       string            `json:"path"`
	Inventory  string            `json:"inventory,omitempty"`
	ExtraVars  map[string]string `json:"extraVars,omitempty"`
	Tags       []string          `json:"tags,omitempty"`
	SkipTags   []string          `json:"skipTags,omitempty"`
	Limit      string            `json:"limit,omitempty"`
	Verbosity  int               `json:"verbosity,omitempty"`
	CheckMode  bool              `json:"checkMode,omitempty"`
	DiffMode   bool              `json:"diffMode,omitempty"`
	Vault      string            `json:"vault,omitempty"`
	PrivateKey string            `json:"privateKey,omitempty"`
	RemoteUser string            `json:"remoteUser,omitempty"`
	BecomeUser string            `json:"becomeUser,omitempty"`
	Timeout    int               `json:"timeout,omitempty"`
}

// Validate validates the playbook manifest configuration.
func (c *AnsiblePlaybookManifest) Validate() error {
	// Path is required
	if c.Path == "" {
		return fmt.Errorf("playbook path is required")
	}

	// Validate verbosity level
	if c.Verbosity < 0 || c.Verbosity > 4 {
		return fmt.Errorf("verbosity must be between 0 and 4, got: %d", c.Verbosity)
	}

	// Validate timeout
	if c.Timeout < 0 {
		return fmt.Errorf("timeout must be non-negative, got: %d", c.Timeout)
	}

	// Validate tags and skip-tags don't overlap
	if len(c.Tags) > 0 && len(c.SkipTags) > 0 {
		tagSet := make(map[string]bool)
		for _, tag := range c.Tags {
			tagSet[tag] = true
		}
		for _, skipTag := range c.SkipTags {
			if tagSet[skipTag] {
				return fmt.Errorf("tag '%s' specified in both tags and skipTags", skipTag)
			}
		}
	}

	// Validate extra vars don't have empty keys
	for key := range c.ExtraVars {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("extra variable key cannot be empty")
		}
	}

	return nil
}

// AnsiblePipelineSpec represents the pipeline configuration for Ansible applications.
type AnsiblePipelineSpec struct {
	Stages []AnsibleStageSpec `json:"stages,omitempty"`
}

// Validate validates the pipeline specification.
func (c *AnsiblePipelineSpec) Validate() error {
	if len(c.Stages) == 0 {
		return fmt.Errorf("pipeline must have at least one stage")
	}

	stageNames := make(map[string]bool)
	for i, stage := range c.Stages {
		if err := stage.Validate(); err != nil {
			return fmt.Errorf("invalid stage %d: %w", i, err)
		}

		// Check for duplicate stage names
		if stageNames[stage.Name] {
			return fmt.Errorf("duplicate stage name: %s", stage.Name)
		}
		stageNames[stage.Name] = true
	}

	return nil
}

// AnsibleStageSpec represents a stage configuration for Ansible applications.
type AnsibleStageSpec struct {
	Name string                 `json:"name"`
	With map[string]interface{} `json:"with,omitempty"`
}

// Validate validates the stage specification.
func (c *AnsibleStageSpec) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("stage name is required")
	}

	// Validate stage name is a known stage
	validStages := []string{"ANSIBLE_SYNC"}
	isValid := false
	for _, validStage := range validStages {
		if c.Name == validStage {
			isValid = true
			break
		}
	}
	if !isValid {
		return fmt.Errorf("unknown stage name: %s, valid stages are: %v", c.Name, validStages)
	}

	return nil
}
