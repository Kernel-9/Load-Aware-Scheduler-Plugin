# Overview

Introduce the actual node load (CPU, memory) collected by the Metrics Server and PodNum as a scoring basis during the scheduling Score phase.

## Load-Aware-Scheduler-Plugin
### Resource Weights
Resources are assigned weights based on the plugin args resources param. The base units for CPU are millicores、for memory are bytes、for podNum are counts.

Example config:

```yaml
apiVersion: kubescheduler.config.k8s.io/v1beta2
kind: KubeSchedulerConfiguration
leaderElection:
  leaderElect: false
clientConnection:
  kubeconfig: "REPLACE_ME_WITH_KUBE_CONFIG_PATH"
profiles:
- schedulerName: default-scheduler
  plugins:
    score:
      enabled:
      - name: NodeResourcesAllocatable
  pluginConfig:
  - name: NodeResourcesAllocatable
    args:
      mode: Least
      resources:
      - name: cpu
        weight: 1000000
      - name: memory
        weight: 1
      - name: podNum
      - weight: 1
```

### Node Resources Least Allocatable
If plugin args specify the priority param "Least", then nodes with the least allocatable resources are scored highest.

### Node Resources Most Allocatable
If plugin args specify the priority param "Most", then nodes with the most allocatable resources are scored highest.
