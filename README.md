# Cloudflare drand node

These are the configuration files for running a [drand](https://github.com/dedis/drand) node on PDX k8s. Ultimately, this Kubernetes service will provide randomness from lavarand to the overall distributed drand beacon. Our drand node will be publicly accessible via drand.cloudflare.com for public use (namely, getting randomness).

## Relevant links

 - [Functional spec](https://wiki.cfops.it/display/CRYPTO/Functional+Specification%3A+Distributed+Randomness+Beacon+Daemon)
 - [JIRA RM](https://jira.cfops.it/browse/RM-3276)

## Building drand image

In the `drand_service` directory, run `make build-container`. You should edit the Makefile to customize the path and tag assigned to your custom drand image. If you keep `/u/gabbi/drand` in the image name, she will find out and heckle you!!!

## Testing

To run the experimental (not production) drand cluster on the drand namespace and ensure that drand requests can make it to the Kubernetes ingresscontroller, this project features a `kubernetes-test.yaml`, which does the following:

1. Launches a **drand service**
2. Launches a **drand pod** that consists of four containers, each running a drand beacon. These pods are configured to use the test group configuration, which is stored in Kubernetes as the group-toml-test configMap. Each container also loads drand_id.public and drand_id.private from Kubernetes secrets defined in the drand namespace. These four drand instances make the the minimum size quorum necessary for running a drand cluster. 
3. Defines a **cloudflare-only ingress** that defines endpoints that can be accessed with the URL drand.ing.pdx-a.k8s.cfplat.com. There is a forced SSL redirect, so just send http and gRPC requests to drand.ing.pdx-a.k8s.cfplat.com:443.
4. Defines a **public ingress** that defines endpoints that can be accessed with the URL drand.cloudflare.com, and the origin CA-signed cert for this subdomain. Send http and gRPC requests to drand.cloudflare.com:443.

## How to use the Cloudflare drand node

Follow the instructions in from the [drand README](https://github.com/dedis/drand), and use drand.ing.pdx-a.k8s.cfplat.com:443 for the node address if you're on VPN, and drand.cloudflare.com:443 if you want to use the public endpoint.
