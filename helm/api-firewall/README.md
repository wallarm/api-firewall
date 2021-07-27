# Helm chart for Wallarm API Firewall

This chart bootstraps Wallarm API Firewall deployment on a [Kubernetes](http://kubernetes.io/) cluster using the [Helm](https://helm.sh/) package manager.

This chart is not uploaded to any public Helm registry yet. To deploy the Helm chart, please use this repository.

## Requirements

* Kubernetes 1.10 or later
* Helm 2.16 or later

## Deployment

To deploy the Wallarm API Firewall Helm chart:

1. Clone this repository.
2. Configure the Helm chart by changing the `helm/api-firewall/values.yaml` file following the code comments.
3. Deploy Wallarm API Firewall from this Helm chart.

To see the example of this Helm chart deployment, you can run our [Kuberentes demo](https://github.com/wallarm/api-firewall/tree/main/demo/kubernetes).
