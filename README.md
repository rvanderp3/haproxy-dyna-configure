# Dynamic HAProxy Configuration for OpenShift and OKD

## Overview
In automated environments where ephemeral clusters come and go it 
can be a challenge to provide access to those clusters if those clusters
aren't using an integrated load balancer. This project aims to create a
simplistic operator that requires no authentication with an OpenShift cluster
to both discover it, and generate targets and SNI routing to that cluster.

## How Does it Work?
An administrator creates a configuration which defines the address ranges and 
ports to monitor:

~~~yaml
monitor-config:
  check-timeout: 500
  monitor-ranges:
    - ip-address-start: "192.168.100.200"
      ip-address-end: "192.168.100.240"
      monitor-ports:
        - port: 6443
          name: "api"
          path-match: "api"
        - port: 443
          name: "ingress-https"
          path-prefix: "*.apps"
    - ip-address-start: "192.168.151.2"
      ip-address-end: "192.168.151.99"
      monitor-ports:
        - port: 6443
          name: "api"
          path-match: "api"
        - port: 443
          name: "ingress-https"
          path-prefix: "*.apps"
    - ip-address-start: "192.168.152.2"
      ip-address-end: "192.168.152.99"
      monitor-ports:
        - port: 6443
          name: "api"
          path-match: "api"
        - port: 443
          name: "ingress-https"
          path-prefix: "*.apps"
    - ip-address-start: "192.168.153.2"
      ip-address-end: "192.168.153.99"
      monitor-ports:
        - port: 6443
          name: "api"
          path-match: "api"
        - port: 443
          name: "ingress-https"
          path-prefix: "*.apps"
    - ip-address-start: "192.168.154.2"
      ip-address-end: "192.168.154.99"
      monitor-ports:
        - port: 6443
          name: "api"
          path-match: "api"
        - port: 443
          name: "ingress-https"
          path-prefix: "*.apps"
    - ip-address-start: "192.168.155.2"
      ip-address-end: "192.168.155.99"
      monitor-ports:
        - port: 6443
          name: "api"
          path-match: "api"
        - port: 443
          name: "ingress-https"
          path-prefix: "*.apps"
~~~

When the operator syncs, it performs a multi-threaded query of the IP ranges to discover
active ingress endpoints. The ingress endpoints are queried and the cluster base domain is 
extracted. This base domain is then used to build SNI routing in the HAProxy configuration.

## Prereqisites

- `expose-fd listeners` is configured on the HAProxy `stats` socket
- The operator is running with a Linux user account capable of reading/writing to the socket
- The operator must be running on the same system as HAProxy
- monitor-config.yaml is configured with the desired ranges and ports.

## Building the Tool

~~~shell
go mod tidy
go mod vendor
./hack/build.sh
~~~

## Running the Tool

Note: At this time, the operator runs as a standalone binary. 

~~~shell
./bin/haproxy-dyna-configure
systemctl reload haproxy
~~~