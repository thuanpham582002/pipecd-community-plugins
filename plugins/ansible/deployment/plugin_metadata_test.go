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
	"testing"

	sdk "github.com/pipe-cd/piped-plugin-sdk-go"
	"go.uber.org/zap"
)

func TestInitialMetadata(t *testing.T) {
	logger := zap.NewNop()

	testCases := []struct {
		name           string
		stageConfig    sdk.StageConfig
		expectedKey    string
		expectedValue  string
	}{
		{
			name: "ANSIBLE_SYNC stage",
			stageConfig: sdk.StageConfig{
				Name: string(AnsibleSync),
			},
			expectedKey:   sdk.MetadataKeyStageDisplay,
			expectedValue: "Deploy Ansible Configuration",
		},
		{
			name: "Unknown stage",
			stageConfig: sdk.StageConfig{
				Name: "UNKNOWN_STAGE",
			},
			expectedKey:   sdk.MetadataKeyStageDisplay,
			expectedValue: "UNKNOWN_STAGE",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metadata, err := initialMetadata(tc.stageConfig, logger)
			if err != nil {
				t.Fatalf("initialMetadata() returned unexpected error: %v", err)
			}

			if value, exists := metadata[tc.expectedKey]; !exists {
				t.Errorf("Expected metadata key %q not found", tc.expectedKey)
			} else if value != tc.expectedValue {
				t.Errorf("Expected metadata value %q, got %q", tc.expectedValue, value)
			}
		})
	}
}

func TestBuildPipelineWithMetadata(t *testing.T) {
	logger := zap.NewNop()

	testCases := []struct {
		name     string
		stages   []sdk.StageConfig
		expected string
	}{
		{
			name:     "Empty stages should create default ANSIBLE_SYNC",
			stages:   []sdk.StageConfig{},
			expected: "Deploy Ansible Configuration",
		},
		{
			name: "Explicit ANSIBLE_SYNC stage",
			stages: []sdk.StageConfig{
				{Name: string(AnsibleSync), Index: 0},
			},
			expected: "Deploy Ansible Configuration",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			stages, err := buildPipeline(tc.stages, false, logger)
			if err != nil {
				t.Fatalf("buildPipeline() returned unexpected error: %v", err)
			}

			if len(stages) == 0 {
				t.Fatal("buildPipeline() returned empty stages")
			}

			displayValue := stages[0].Metadata[sdk.MetadataKeyStageDisplay]
			if displayValue != tc.expected {
				t.Errorf("Expected stage display metadata %q, got %q", tc.expected, displayValue)
			}
		})
	}
}