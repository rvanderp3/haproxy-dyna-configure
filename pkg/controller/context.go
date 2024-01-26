package controllers

import (
	"fmt"
	"strings"
	"sync"

	"github.com/openshift-splat-team/haproxy-dyna-configure/data"
	"github.com/openshift-splat-team/haproxy-dyna-configure/pkg/util"
	corev1 "k8s.io/api/core/v1"
)

var (
	targetsMutex sync.Mutex
)

type ControllerContext struct {
	namespaceTargets map[string]map[string]*data.NamespaceTarget
	config           *data.MonitorConfig
}

func getEnvFromPod(pod *corev1.Pod, varName string) (string, error) {
	for _, container := range pod.Spec.Containers {
		envVars := container.Env
		for _, envVar := range envVars {
			if envVar.Name == varName {
				return envVar.Value, nil
			}
		}
	}

	return "", fmt.Errorf("unable to find envvar %s", varName)

}
func (c *ControllerContext) Initialize(config *data.MonitorConfig) {
	c.namespaceTargets = map[string]map[string]*data.NamespaceTarget{}
	c.config = config
}

func (c *ControllerContext) Update(pod *corev1.Pod) error {
	ns := pod.Namespace
	if !strings.HasPrefix(ns, "ci-op-") {
		return nil
	}
	var jobHash string
	jobHash, err := getEnvFromPod(pod, "JOB_NAME_HASH")
	if err != nil {
		return fmt.Errorf("no job hash is associated with this pod: %v", err)
	}

	targetsMutex.Lock()
	defer targetsMutex.Unlock()

	var exists bool
	var namespaceTarget map[string]*data.NamespaceTarget
	if namespaceTarget, exists = c.namespaceTargets[ns]; !exists {
		namespaceTarget = map[string]*data.NamespaceTarget{}
	}

	if _, exists := namespaceTarget[jobHash]; !exists {
		namespaceTarget[jobHash] = &data.NamespaceTarget{
			Namespace: ns,
			JobHash:   jobHash,
		}
	}
	c.namespaceTargets[ns] = namespaceTarget
	return nil
}

func (c *ControllerContext) CheckForARecords() error {
	hostsToCheck := map[string]string{}
	targetHostCheckMap := map[string]*data.NamespaceTarget{}

	targetsMutex.Lock()
	for ns, namespaceTarget := range c.namespaceTargets {
		for hashId, target := range namespaceTarget {
			if len(target.APIVIP) == 0 {
				url := fmt.Sprintf("api.%s-%s.%s", ns, hashId, c.config.BaseDomain)
				hostsToCheck[url] = ""
				targetHostCheckMap[url] = target
			}
			if len(target.IngressVIP) == 0 {
				url := fmt.Sprintf("*.apps.%s-%s.%s", ns, hashId, c.config.BaseDomain)
				hostsToCheck[url] = ""
				targetHostCheckMap[url] = target
			}
		}
	}
	targetsMutex.Unlock()

	for host := range hostsToCheck {
		ips, err := util.ResolveHost(host)
		if err != nil {
			// unable to resolve host, continue
			continue
		}
		for _, ip := range ips {
			ipStr := ip.String()
			if !strings.HasPrefix(ipStr, "10.") {
				continue
			}
			hostsToCheck[host] = ipStr
		}
	}

	targetsMutex.Lock()
	defer targetsMutex.Unlock()

	for host, ip := range hostsToCheck {
		if len(ip) == 0 {
			continue
		}
		var target *data.NamespaceTarget
		var exists bool
		if target, exists = targetHostCheckMap[host]; !exists {
			continue
		}
		if strings.HasPrefix(host, "api.") {
			target.APIVIP = ip
		} else {
			target.IngressVIP = ip
		}
		c.namespaceTargets[target.Namespace][target.JobHash] = target
	}
	return nil
}

func (c *ControllerContext) Destroy(namespace *corev1.Namespace) error {
	ns := namespace.Name

	targetsMutex.Lock()
	defer targetsMutex.Unlock()
	if _, exists := c.namespaceTargets[ns]; !exists {
		delete(c.namespaceTargets, ns)
	}
	return nil
}
