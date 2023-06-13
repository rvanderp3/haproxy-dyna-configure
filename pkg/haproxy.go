package pkg

import (
	"fmt"
	"strings"

	clientnative "github.com/haproxytech/client-native"
	"github.com/haproxytech/models"
	"github.com/rvanderp3/haproxy-dyna-configure/data"
	"github.com/sirupsen/logrus"
)

func makeCleanModel(client *clientnative.HAProxyClient) error {
	config := client.Configuration
	_, frontEnds, err := config.GetFrontends("")
	if err != nil {
		return err
	}

	for _, frontEnd := range frontEnds {
		version, err := config.GetVersion("")
		if err != nil {
			return err
		}
		if frontEnd.Name == "stats" {
			continue
		}
		err = client.Configuration.DeleteFrontend(frontEnd.Name, "", version)
		if err != nil {
			return err
		}
	}

	_, backEnds, err := client.Configuration.GetBackends("")
	if err != nil {
		return err
	}
	for _, backEnd := range backEnds {
		version, err := config.GetVersion("")
		if err != nil {
			return err
		}
		err = client.Configuration.DeleteBackend(backEnd.Name, "", version)
		if err != nil {
			return err
		}
	}
	return nil
}

func createFrontend(client *clientnative.HAProxyClient, name string, port *data.MonitorPort, bindAddress string) error {
	logrus.Infof("creating frontend %s", name)
	config := client.Configuration

	version, err := config.GetVersion("")
	if err != nil {
		return err
	}

	fe := models.Frontend{
		Mode: models.FrontendModeTCP,
		Name: name,
	}

	_, _, err = config.GetFrontend(name, "")
	if err == nil {
		logrus.Infof("frontend %s already exists", name)
		return nil
	}

	err = config.CreateFrontend(&fe, "", version)
	if err != nil {
		return err
	}

	version++
	id := int64(0)
	timeout := int64(5000)
	tcpRule1 := models.TCPRequestRule{
		ID:      &id,
		Type:    models.TCPRequestRuleTypeInspectDelay,
		Timeout: &timeout,
	}
	err = config.CreateTCPRequestRule("frontend", name, &tcpRule1, "", version)
	if err != nil {
		return err
	}

	version++
	tcpRule2 := models.TCPRequestRule{
		Action:   models.TCPRequestRuleActionAccept,
		Cond:     models.TCPRequestRuleCondIf,
		ID:       &id,
		CondTest: "{ req_ssl_hello_type 1 }",
		Type:     models.TCPRequestRuleTypeContent,
	}
	err = config.CreateTCPRequestRule("frontend", name, &tcpRule2, "", version)
	if err != nil {
		return err
	}

	version++
	if len(bindAddress) == 0 {
		bindAddress = "0.0.0.0"
	}
	bind := models.Bind{
		Address: bindAddress,
		Port:    &port.Port,
		Name:    name,
	}
	err = config.CreateBind(name, &bind, "", version)
	if err != nil {
		return err
	}
	return nil
}

func createBackendSwitchingRule(client *clientnative.HAProxyClient, baseDomain string, frontendName string, backendName string, port *data.MonitorPort) error {
	logrus.Infof("creating backend switching rule %s", backendName)
	config := client.Configuration
	version, err := config.GetVersion("")
	if err != nil {
		return err
	}
	id := int64(0)

	var rule models.BackendSwitchingRule

	if len(port.PathPrefix) > 0 {
		pathPrefix := port.PathPrefix
		if strings.HasPrefix(pathPrefix, "*") {
			pathPrefix = pathPrefix[1:]
		}
		rule = models.BackendSwitchingRule{
			Cond:     "if",
			ID:       &id,
			Name:     backendName,
			CondTest: fmt.Sprintf("{ req.ssl_sni -m end %s%s }", pathPrefix, baseDomain),
		}
	} else if len(port.PathMatch) > 0 {
		rule = models.BackendSwitchingRule{
			Cond:     "if",
			ID:       &id,
			Name:     backendName,
			CondTest: fmt.Sprintf("{ req.ssl_sni -i %s%s }", port.PathMatch, baseDomain),
		}
	}
	err = config.CreateBackendSwitchingRule(frontendName, &rule, "", version)
	if err != nil {
		return err
	}

	return nil
}

func createBackend(client *clientnative.HAProxyClient, name string, port *data.MonitorPort) error {
	logrus.Infof("creating backend %s", name)
	config := client.Configuration
	version, err := config.GetVersion("")
	if err != nil {
		return err
	}
	backend := &models.Backend{
		Mode: models.BackendModeTCP,
		Name: name,
	}
	err = config.CreateBackend(backend, "", version)
	if err != nil {
		return err
	}

	for _, target := range port.Targets {
		toPort := port.Port
		server := &models.Server{
			Address: target,
			Port:    &toPort,
			Name:    fmt.Sprintf("%s-%d", target, toPort),
			Check:   models.ServerCheckEnabled,
			Verify:  models.ServerVerifyNone,
		}
		version, err = config.GetVersion("")
		if err != nil {
			return err
		}
		err = config.CreateServer(name, server, "", version)
		if err != nil {
			return err
		}
	}
	return nil
}

func ApplyDiscoveredConfiguration(monitorConfig *data.MonitorConfigSpec) error {
	client, err := clientnative.DefaultClient()
	if err != nil {
		return err
	}

	err = makeCleanModel(client)
	if err != nil {
		return err
	}
	for _, monitorRange := range monitorConfig.MonitorConfig.MonitorRanges {
		for _, monitorPort := range monitorRange.MonitorPorts {
			if len(monitorPort.Targets) == 0 || len(monitorRange.BaseDomain) == 0 {
				continue
			}
			name := fmt.Sprintf("%s-%d", monitorRange.BaseDomain, monitorPort.Port)
			frontendName := fmt.Sprintf("dyna-frontend-%d", monitorPort.Port)
			err := createBackend(client, name, &monitorPort)
			if err != nil {
				return err
			}
			err = createFrontend(client, frontendName, &monitorPort, "0.0.0.0")
			if err != nil {
				return err
			}
			err = createBackendSwitchingRule(client, monitorRange.BaseDomain, frontendName, name, &monitorPort)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func ApplyHypershiftConfiguration() error {
	client, err := clientnative.DefaultClient()
	if err != nil {
		return err
	}

	hypershiftContext := GetHypershiftContext()

	for _, cluster := range hypershiftContext.HostedClusters {
		for _, port := range cluster.ControlPlanePorts {
			frontendName := fmt.Sprintf("dyna-frontend-%d", port)
			backendName := fmt.Sprintf("%s-%d", cluster.BaseDomain, port)

			apiTargets := []string{}

			for _, target := range hypershiftContext.HostingNodesIPs {
				apiTargets = append(apiTargets, target)
			}
			apiPort := &data.MonitorPort{
				Port:       int64(port),
				Name:       frontendName,
				Targets:    apiTargets,
				PathPrefix: "api",
			}
			err = createBackend(client, backendName, apiPort)
			if err != nil {
				return err
			}
			bindAddress := "0.0.0.0"
			if port == cluster.IgnitionPort {
				ignitionIP := GetIgnitionBindIP()
				if len(ignitionIP) > 0 {
					bindAddress = ignitionIP
				}
			}
			err = createFrontend(client, frontendName, apiPort, bindAddress)
			if err != nil {
				return err
			}
			err = createBackendSwitchingRule(client, cluster.BaseDomain, frontendName, backendName, apiPort)
			if err != nil {
				return err
			}
		}

		ingressPort := 443
		frontendName := "dyna-frontend-443"
		backendName := fmt.Sprintf("%s-443", cluster.BaseDomain)
		ingressTargets := []string{}

		for _, target := range cluster.ComputeNodeIP {
			ingressTargets = append(ingressTargets, target)
		}
		ingressMonitorPort := &data.MonitorPort{
			Port:       int64(ingressPort),
			Name:       frontendName,
			Targets:    ingressTargets,
			PathPrefix: "apps",
		}
		err = createBackend(client, backendName, ingressMonitorPort)
		if err != nil {
			return err
		}
		err = createFrontend(client, frontendName, ingressMonitorPort, "0.0.0.0")
		if err != nil {
			return err
		}
		err = createBackendSwitchingRule(client, cluster.BaseDomain, frontendName, backendName, ingressMonitorPort)
		if err != nil {
			return err
		}
	}
	return nil
}
