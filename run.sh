#!/bin/sh
    
cp /config/monitor-config.yaml .

while true; do
    oc get secret -n test-credentials vsphere-config -o=jsonpath='{.data.subnets\.json}' | base64 -d > /tmp/subnets.json
    ./bin/haproxy-dyna-configure
    sleep 60
done