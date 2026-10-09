# OpenSN playbook guide

## Prerequisites

- A `worker` group containing all hosts. They will be used for running worker nodes during an experiment
- A `control` group containing a single host. It will be used for running the user-defined configurator, main node and common services
- Hosts must be able to execute sudo commands, e.g., by adding `-K` flag when executing playbooks or using ansible vault
- It is assumed all hosts have docker pre-installed and the ansible user is part of the `docker` group
- The container images must be available to all hosts (e.g., manual distribution, shared registry)

## Configuration

Following variables for role `opensn` have to be configured:

- `opensn_remote_working_dir`: Folder that will be used as working directory for running OpenSN
- `opensn_script_source_directory`: Folder containing scripts `delete_link_prefix.sh`, `kernel_setting,sh`, `start_depend_service.sh`, and `stop_depend_service.sh`
- `opensn_daemon_src`: Local source path to the `NodeDaemon` executable
- `opensn_host_config_template_src`: Local source path to the config for the `NodeDaemon`. A template is provided in `template/host_config.json.j2`

The example `run_experiment.yml` requires:
- `opensn_experiment_controler_src`: Local source path to the experiment controler using OpenSN's API
- `opensn_topology_config_src`: Local source path to the topology config
- `opensn_node_config_src`: Local source path to the node config

## Workflow

1. Configure a suitable environment with `setup_environment.yml`
2. Run an experiment. An example playbook is provided in `run_experiment.yml`
3. Cleanup the environment with `teardown_environment.yml` after the experiment has finished and all relevant data got extracted
