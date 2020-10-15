# machine-api

The Machine API is a declarative way of managing infrastructure resources using Kubernetes in-cluster resources (CRDs). It was originally forked from [Cluster API](https://github.com/kubernetes-sigs/cluster-api) without cluster-level resource management, and an ability to operate on existing cluster resources (i.e. machines). This repo is the "core" of the Machine API and is installed in each cluster that will use the Machine API. The infrastructure providers, such as [machine-api-provider-docker](https://github.com/criticalstack/machine-api-provider-docker) or [machine-api-provider-aws](https://github.com/criticalstack/machine-api-provider-aws), are only installed on clusters where that provider will be used to manage infrastructure. The machine-api-provider-docker, or mapd, is a special infrastructure provider in that it is abstracting local docker using [cinder](https://docs.crit.sh/cinder-guide/what-is-cinder.html) instead of a cloud-provider. The idea here is this enables developing Machine API providers easily by exposing the same declarative interface locally.


## Getting started


Create a new cinder cluster, enabling the machine-api to be installed:

```shell
$ cinder create cluster -c - << EOT
apiVersion: cinder.crit.sh/v1alpha1
kind: ClusterConfiguration
featureGates:
  MachineAPI: true
EOT
```

Check out the nodes running:

```shell
$ kubectl get no

NAME              STATUS   ROLES    AGE   VERSION
cinder            Ready    master   30s   v1.18.5
```


Create a simple worker configuration by applying the following CRD:

```yaml
apiVersion: machine.crit.sh/v1alpha1
kind: Config
metadata:
  name: worker-config
spec:
  config:
    apiVersion: crit.sh/v1alpha2
    kind: WorkerConfiguration
    clusterName: cinder
```

To create a new worker machine using the above worker configuration, now apply a new `Machine` and `DockerMachine`:

```yaml
apiVersion: machine.crit.sh/v1alpha1
kind: Machine
metadata:
  name: worker-1
spec:
  configRef:
    name: worker-config
  infrastructureRef:
    apiVersion: infrastructure.crit.sh/v1alpha1
    kind: DockerMachine
    name: worker-1
---
apiVersion: infrastructure.crit.sh/v1alpha1
kind: DockerMachine
metadata:
  name: worker-1
spec:
  clusterName: cinder
  containerName: cinder-worker-1
  image: docker.io/criticalstack/cinder:v1.0.3
```

The `Machine` is the abstract that is used to represent the idea of a virtual machine, and the `DockerMachine` is the provider-specific implementation of the virtual machine that is used to specify any provider-specific configuration. For example, docker uses container images (vs. something like AWS that uses AMIs), so it allows configuration of a container image in it's CRD.


## List of infrastructure providers

* [machine-api-provider-docker](https://github.com/criticalstack/machine-api-provider-docker)
* [machine-api-provider-aws](https://github.com/criticalstack/machine-api-provider-aws)
