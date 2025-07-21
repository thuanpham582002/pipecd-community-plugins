# Ansible Plugin

| Metadata        |           |
| ------------- |-----------|
|[Stability](/README.md#stability-levels)     | In Development   |
| Issues        | [![Open issues](https://img.shields.io/github/issues-search/pipe-cd/community-plugins?query=is%3Aissue%20is%3Aopen%20label%3Aplugin%2Fansible%20&label=open&color=orange)](https://github.com/pipe-cd/community-plugins/issues?q=is%3Aopen+is%3Aissue+label%3Aplugin%2Fansible) |
| [Code Owners](/CONTRIBUTING.md#becoming-a-code-owner)   |  [@ntheanh201](https://github.com/@ntheanh201)  |

## Supported Features

- ✅ QuickSync
- ✅ PipelineSync  
- ✅ Multi-target deployment
- ❌ Prune
- ❌ LiveState View
- ❌ DriftDetection
- ❌ PlanPreview

## Overview

The Ansible plugin enables deployment of applications using Ansible playbooks within PipeCD. It provides comprehensive configuration options for SSH connections, environment variables, timeouts, and command-line flags. The plugin supports both QuickSync and PipelineSync strategies and can deploy to multiple targets simultaneously.

## Stages

### ANSIBLE_SYNC stage

This stage executes an Ansible playbook with the specified configuration. It supports all standard ansible-playbook command-line options including verbosity levels, tags, extra variables, vault integration, and SSH configuration.

## Plugin Configuration

### Plugin scope config

Currently, the Ansible plugin does not require any plugin-level configuration.

```yaml
kind: Piped
spec:
    plugins:
      - name: ansible
        port: 8080
        config: {}  # No plugin-level config required
        deployTargets: 
          - name: production
            ...
```

### Deploy Target config

Deploy Target configuration provides environment-specific settings for Ansible deployments:

```yaml
kind: Piped
spec:
    plugins:
      - name: ansible
        port: 8080
        deployTargets: 
          - name: production
            labels:
              env: prod
            config:
              ansiblePath: "/usr/local/bin/ansible-playbook"
              inventory: "production/hosts"
              vault: "production/vault_pass"
              connectionTimeout: 30
              commandTimeout: 1800
              hostKeyChecking: false
              workingDirectory: "ansible"
              env:
                ANSIBLE_HOST_KEY_CHECKING: "False"
                ANSIBLE_GATHERING: "smart"
              ssh:
                user: "deploy"
                privateKeyFile: "/etc/piped-secret/ssh-key"
                port: 22
                connectionAttempts: 3
                controlPersist: "5m"
              extraFlags:
                - "--strategy=linear"
                - "--forks=10"
```

| Field | Type | Description | Required | Default |
|-|-|-|-|-|
| ansiblePath | string | Path to ansible-playbook executable | No | "ansible-playbook" |
| inventory | string | Default inventory file path (relative to app directory) | No | |
| vault | string | Default vault password file path (relative to app directory) | No | |
| workingDirectory | string | Working directory for ansible commands | No | Application directory |
| env | map[string]string | Environment variables for ansible commands | No | |
| ssh | [AnsibleSSHConfig](#ansiblesshconfig) | SSH connection configuration | No | |
| connectionTimeout | int | SSH connection timeout in seconds | No | 0 (no timeout) |
| commandTimeout | int | Command execution timeout in seconds | No | 0 (no timeout) |
| hostKeyChecking | bool | Enable SSH host key checking | No | true |
| extraFlags | []string | Additional command-line flags for ansible-playbook | No | |

#### AnsibleSSHConfig

| Field | Type | Description | Required | Default |
|-|-|-|-|-|
| user | string | SSH username | No | |
| privateKeyFile | string | SSH private key file path (relative to app directory) | No | |
| port | int | SSH port number | No | 22 |
| connectionAttempts | int | Number of SSH connection attempts | No | 1 |
| controlPersist | string | SSH connection multiplexing control persist time | No | |

## Application Configuration

### Application scope options

Configure Ansible playbook execution for your application:

```yaml
apiVersion: pipecd.dev/v1beta1
kind: Application
metadata:
  name: my-ansible-app
spec:
  plugins:
    ansible:
      name: "My Ansible Application"
      playbook:
        path: "site.yml"
        inventory: "inventory/production"
        verbosity: 2
        extraVars:
          app_version: "v1.2.3"
          environment: "production"
          debug_mode: "false"
        tags:
          - "deploy"
          - "configure"
        skipTags:
          - "backup"
        limit: "webservers"
        checkMode: false
        diffMode: true
        vault: "vault/production_vault_pass"
        privateKey: "keys/deploy_key"
        remoteUser: "deploy"
        becomeUser: "root"
        timeout: 1800
      pipeline:
        stages:
          - name: ANSIBLE_SYNC
            with:
              timeout: 3600
```

| Field | Type | Description | Required | Default |
|-|-|-|-|-|
| name | string | Application name for logging | No | |
| playbook | [AnsiblePlaybookManifest](#ansibleplaybookmanifest) | Playbook configuration | Yes | |
| pipeline | [AnsiblePipelineSpec](#ansiblepipelinespec) | Pipeline configuration | No | |

#### AnsiblePlaybookManifest

| Field | Type | Description | Required | Default |
|-|-|-|-|-|
| path | string | Path to playbook file (relative to app directory) | Yes | |
| inventory | string | Inventory file path (overrides deploy target default) | No | |
| extraVars | map[string]string | Extra variables for ansible-playbook | No | |
| tags | []string | Tags to run | No | |
| skipTags | []string | Tags to skip | No | |
| limit | string | Limit execution to specific hosts/groups | No | |
| verbosity | int | Verbosity level (0-4) | No | 0 |
| checkMode | bool | Run in check mode (dry-run) | No | false |
| diffMode | bool | Show diffs for changed files | No | false |
| vault | string | Vault password file path (overrides deploy target default) | No | |
| privateKey | string | SSH private key file path (overrides deploy target SSH config) | No | |
| remoteUser | string | Remote SSH user (overrides deploy target SSH config) | No | |
| becomeUser | string | User to become via sudo/su | No | |
| timeout | int | Command timeout in seconds (overrides deploy target default) | No | 0 |

#### AnsiblePipelineSpec

| Field | Type | Description | Required | Default |
|-|-|-|-|-|
| stages | []AnsibleStageSpec | Pipeline stages | Yes | |

#### AnsibleStageSpec

| Field | Type | Description | Required | Default |
|-|-|-|-|-|
| name | string | Stage name (must be "ANSIBLE_SYNC") | Yes | |
| with | map[string]interface{} | Stage-specific configuration | No | |

### Stage options

#### ANSIBLE_SYNC stage

| Field | Type | Description | Required | Default |
|-|-|-|-|-|
| timeout | int | Stage execution timeout in seconds | No | 0 (no timeout) |

## Usage Examples

### Basic QuickSync Deployment

```yaml
apiVersion: pipecd.dev/v1beta1
kind: Application
spec:
  plugins:
    ansible:
      playbook:
        path: "deploy.yml"
        inventory: "hosts"
        verbosity: 1
```

### Pipeline Deployment with Multiple Stages

```yaml
apiVersion: pipecd.dev/v1beta1
kind: Application
spec:
  plugins:
    ansible:
      playbook:
        path: "site.yml"
        inventory: "production/hosts"
        extraVars:
          app_version: "{{ .Input.version }}"
      pipeline:
        stages:
          - name: ANSIBLE_SYNC
            with:
              timeout: 1800
```

### Multi-Target Deployment

Deploy the same playbook to multiple environments by configuring multiple deploy targets in the Piped configuration:

```yaml
# Piped configuration
kind: Piped
spec:
  plugins:
    - name: ansible
      deployTargets:
        - name: staging
          labels:
            env: staging
          config:
            inventory: "staging/hosts"
            hostKeyChecking: false
        - name: production
          labels:
            env: production
          config:
            inventory: "production/hosts"
            connectionTimeout: 60
```

## Prerequisites

1. **Ansible Installation**: Ensure Ansible is installed on the Piped agent system
2. **Python**: Ansible requires Python 2.7 or 3.5+
3. **SSH Access**: Configure SSH access to target hosts
4. **Inventory Files**: Prepare Ansible inventory files
5. **Playbooks**: Create Ansible playbooks in your application repository

## Security Considerations

- Store sensitive data like vault passwords and SSH keys in PipeCD secrets
- Use vault encryption for sensitive variables in playbooks
- Consider disabling SSH host key checking only in trusted environments
- Limit SSH user permissions and use sudo/become for privilege escalation
- Regularly rotate SSH keys and vault passwords

## Troubleshooting

### Common Issues

1. **Playbook not found**: Ensure the playbook path is relative to the application directory
2. **SSH connection failures**: Check SSH configuration, keys, and network connectivity
3. **Vault decryption errors**: Verify vault password file location and permissions
4. **Timeout errors**: Increase timeout values for long-running playbooks
5. **Permission denied**: Check SSH user permissions and become configuration

### Debug Mode

Enable verbose logging by setting verbosity level:

```yaml
playbook:
  verbosity: 3  # 0-4, higher values provide more detail
```

### Log Analysis

The plugin provides detailed logging with emojis for easy identification:

- 🚀 Deployment start/completion
- 📁 File and directory operations  
- 🔧 Configuration and command building
- ⏰ Timing and timeout information
- 🔑 SSH and security operations
- ❌ Errors and failures
- ✅ Success confirmations