# Wallarm API Firewall chart 

Light-weighted Wallarm API Firewall protects your API endpoints in cloud-native
environments with API schema validation. Wallarm API Firewall relies on a positive
security model allowing calls that match a predefined API specification, while
rejecting everything else.

API Firewall works as a reverse proxy with a built-in OpenAPI 3.0 request and
response validator. The validator is written in Go and optimized for extreme
performance and near-zero added latency.

This helm chart allows to install API Firewall on Kubernetes cluster in middleware
scheme.

## Introduction

This chart bootstraps an API Firewall deployment on a [Kubernetes](http://kubernetes.io)
cluster using the [Helm](https://helm.sh) package manager.

This chart doesn't present in any public helm registry yet. Build it from scratch, or
use this repository.

## Prerequisites

- Kubernetes 1.10+
- Helm 2.16+

## Configuration

The following table lists the configurable parameters of the api-firewall chart and their default values.

Parameter | Description | Default
--- | --- | ---
`manifest.enabled` | Explicitly define OpenAPI manifest | true
`manifest.body` | OpenAPI v3 manifest in JSON format | empty, required if `manifest.enabled` is true
`apiFirewall.nameOverride` | Name to use instead generated name | empty
`apiFirewall.image.registry` | Registry to use for pulling image | ""
`apiFirewall.image.name` | Name of image | wallarm/api-firewall
`apiFirewall.image.tag` | Tag of image | latest
`apiFirewall.image.pullPolicy` | Image pull policy | IfNotPresent
`apiFirewall.imagePullSecrets` | Optional array of imagePullSecrets containing private registry credentials | empty array
`apiFirewall.config.listenAddress` | Listen address | 0.0.0.0
`apiFirewall.config.listenPort` | Listen port | 80
`apiFirewall.config.maxConnsPerHost` | Max concurent requests for handling by single pod | 512
`apiFirewall.config.timeouts.dial` | Connection dial timeout | 200ms
`apiFirewall.config.timeouts.readFromBackend` | Backend connection read timeout | 5s
`apiFirewall.config.timeouts.writeToBackend` | Backend connection write timeout | 5s
`apiFirewall.config.validationMode.request` | API Firewall mode for requests. Can be BLOCK, LOG_ONLY or DISABLE | "block"
`apiFirewall.config.validationMode.response` | API Firewall mode for response. Can be BLOCK, LOG_ONLY or DISABLE | "block"
`apiFirewall.replicaCount` | Initial replicas for Deployment | 3
`apiFirewall.updateStrategy` | The update strategy to apply to the Deployment | empty
`apiFirewall.minReadySeconds` | Interval between discrete pods transitions | 0
`apiFirewall.revisionHistoryLimit` | Rollback limit | 10
`apiFirewall.podLabels` | Labels to add to the pod metadata | empty map
`apiFirewall.podAnnotations` | Annotations to add to the pod metadata | empty map
`apiFirewall.extraArgs` | Additional command line arguments to pass to api-firewall. Overrides configuration defined by environments variables. Commonly not required. | empty array
`apiFirewall.extraEnvs` | Additional environment variables to set | empty array
`apiFirewall.tolerations` | Node tolerations for server scheduling to nodes with taints | empty array
`apiFirewall.affinity` | Node affinity and anti-affinity | empty
`apiFirewall.nodeSelector` | Node labels for pod assignment | empty map
`apiFirewall.lifecycle` | Lifecycle hooks | empty
`apiFirewall.livenessProbe` | Liveness probe config | empty
`apiFirewall.readinessProbe` | Readiness probe config | empty
`apiFirewall.terminationGracePeriodSeconds` | Graceful shutdown timeout | 60
`apiFirewall.priorityClassName` | Priority class for pods | empty
`apiFirewall.runtimeClassName` | Runtime class for pods | empty
`apiFirewall.securityContext` | Security Context policies for api-firewall container | empty
`apiFirewall.resources` | Pod resources for scheduling/limiting | empty
`apiFirewall.extraContainers` | Additional containers to be added to the pods | empty array
`apiFirewall.extraInitContainers` | Containers, which are run before the app containers are started | empty array
`apiFirewall.extraVolumeMounts` | Additional volumeMounts to the main container | empty array
`apiFirewall.extraVolumes` | Additional volumes to the pods | empty array
`apiFirewall.target.type` | Type of target backend service. Can be "service" or "endpoints" | "service"
`apiFirewall.target.name` | Service name (existing or to create) | empty, required if target type "service"
`apiFirewall.target.port` | Destination port | 80
`apiFirewall.target.endpoints` | Endpoints for attaching for created service | empty array
`apiFirewall.target.annotations` | Annotations for created Service | empty map
`apiFirewall.target.clusterIP` | Cluster IP address of created Service | empty
`apiFirewall.service.type` | Type of Service | ClusterIP
`apiFirewall.service.port` | Service port | 80
`apiFirewall.service.nodePort` |  Service node port (in case of type "NodePort") | empty
`apiFirewall.service.loadBalancerIP` | Load balancer IP (in case of type "LoadBalancer") | empty
`apiFirewall.service.loadBalancerSourceRanges` | Load balancer source ranges (in case of type "LoadBalancer") | empty array
`apiFirewall.service.externalTrafficPolicy` | Load balancer source ranges (in case of type "LoadBalancer" or "NodePort") | empty array
`apiFirewall.service.annotations` | Annotations for created Service | empty map
`apiFirewall.service.clusterIP` | Cluster IP address of created Service. Can be "None" for headless service | empty
`apiFirewall.ingress.enabled` | Enable Ingress object | false
`apiFirewall.ingress.ingressClass` | Ingress class to use | empty
`apiFirewall.ingress.hosts` | Ingress hosts | empty array, required if Ingess enabled
`apiFirewall.ingress.path` | Ingress route | "/"
`apiFirewall.ingress.tls` | TLS configuration | empty array
`apiFirewall.ingress.annotations` | Annotations for created Service | empty map
`apiFirewall.podSecurityPolicy.enabled` | Enable Pod Security Policy restrictions | false
`apiFirewall.podSecurityPolicy.allowedCapabilities` | Pod Security Policy spec.allowedCapabilities value | empty array
`apiFirewall.podSecurityPolicy.privileged` | Pod Security Policy spec.privileged value | false
`apiFirewall.podSecurityPolicy.allowPrivilegeEscalation` | Pod Security Policy spec.allowPrivilegeEscalation value | false
`apiFirewall.podSecurityPolicy.volumes` | Pod Security Policy spec.volumes value | ['configMap', 'emptyDir', 'downwardAPI', 'secret']
`apiFirewall.podSecurityPolicy.hostNetwork` | Pod Security Policy spec.hostNetwork value | false
`apiFirewall.podSecurityPolicy.hostIPC` | Pod Security Policy spec.hostIPC value | false
`apiFirewall.podSecurityPolicy.hostPID` | Pod Security Policy spec.hostPID value | false
`apiFirewall.podSecurityPolicy.runAsUser` | Pod Security Policy spec.runAsUser value | {"rule": "MustRunAsNonRoot"}
`apiFirewall.podSecurityPolicy.supplementalGroups` | Pod Security Policy spec.supplementalGroups value | {"rule": "MustRunAs", "ranges": [{"min": 1, "max": 65535}]}
`apiFirewall.podSecurityPolicy.fsGroup` | Pod Security Policy spec.fsGroup value | {"rule": "MustRunAs", "ranges": [{"min": 1, "max": 65535}]}
`apiFirewall.podSecurityPolicy.seLinux` | Pod Security Policy spec.fsGroup value | {"rule": "RunAsAny"}
`apiFirewall.podSecurityPolicy.additionalRestrictions` | Pod Security Policy other values | empty map
`apiFirewall.podDisruptionBudget.enabled` | Enable Pod Disturion Budget | true
`apiFirewall.podDisruptionBudget.maxUnavailable` | Pod Disturion Budget spec.maxUnavailable value | 1
`apiFirewall.autoscaling.enabled` | Enable Autoscaling policy | false
`apiFirewall.autoscaling.minReplicas` | Min count of pods for HPA | 3
`apiFirewall.autoscaling.maxReplicas` | Max count of pods for HPA | 3
`apiFirewall.autoscaling.targetCPUUtilizationPercentage` | CPU threshold for scaling for HPA, in percents | 70
`apiFirewall.autoscaling.targetMemoryUtilizationPercentage` | Memory threshold for scaling for HPA, in percents | 70
`apiFirewall.serviceAccount.name` | Creates new ServiceAccount if empty string | empty, means generate name
`apiFirewall.serviceAccount.name` | Annotations for created ServiceAccount | empty map

## Usage example

See our awesome kubernetes demo here: https://github.com/wallarm/api-firewall/tree/main/demo/kubernetes
