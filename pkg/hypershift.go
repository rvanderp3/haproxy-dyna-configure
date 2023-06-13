package pkg

import (
	"context"
	"flag"
	goflag "flag"
	"os"
	"strings"
	"sync"

	capi "github.com/rvanderp3/haproxy-dyna-configure/third_party/cluster-api/api/v1beta1"
	hypershift "github.com/rvanderp3/haproxy-dyna-configure/third_party/hypershift/api/v1beta1"

	log "github.com/sirupsen/logrus"
	flags "github.com/spf13/pflag"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	osclientset "github.com/openshift/client-go/config/clientset/versioned"
)

const (
	clusterNameLabel = "cluster.x-k8s.io/cluster-name"
)

var (
	hypershiftContextMu sync.Mutex
	hypershiftContext   = HypershiftContext{
		HostedClusters:  map[string]*HostedCluster{},
		HostingNodesIPs: map[string]string{},
	}
)

type HostedControlPlaneController struct {
	client.Client
}

type MachineController struct {
	client.Client
}

type HypershiftContext struct {
	HostingNodesIPs map[string]string
	HostedClusters  map[string]*HostedCluster
}

type HostedCluster struct {
	ApiFQDN           string
	ControlPlanePorts []int32
	BaseDomain        string
	ComputeNodeIP     map[string]string
}

func StartManager() error {
	loggerOpts := &logzap.Options{
		Development: true, // a sane default
		ZapOpts:     []zap.Option{zap.AddCaller()},
	}
	{
		var goFlagSet goflag.FlagSet
		loggerOpts.BindFlags(&goFlagSet)
		flags.CommandLine.AddGoFlagSet(&goFlagSet)
	}
	flag.Parse()
	ctrl.SetLogger(logzap.New(logzap.UseFlagOptions(loggerOpts)))
	ctrl.Log.Info("Starting...")

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		log.Error(err, "could not create manager")
		os.Exit(1)
	}

	hypershift.AddToScheme(mgr.GetScheme())
	capi.AddToScheme(mgr.GetScheme())

	osclientset.NewForConfig(config.GetConfigOrDie())

	err = builder.
		ControllerManagedBy(mgr). // Create the ControllerManagedBy
		For(&hypershift.HostedControlPlane{}).
		Complete(&HostedControlPlaneController{
			Client: mgr.GetClient(),
		})
	if err != nil {
		log.Error(err, "could not create controller")
		os.Exit(1)
	}
	log.Infof("starting controller manager")
	err = builder.
		ControllerManagedBy(mgr). // Create the ControllerManagedBy
		For(&capi.Machine{}).
		Complete(&MachineController{
			Client: mgr.GetClient(),
		})
	if err != nil {
		log.Error(err, "could not create controller")
		os.Exit(1)
	}

	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "could not start manager")
		os.Exit(1)
	}
	return nil
}

// GetHostedClusters returns a deep copy of known hosted clusters
func GetHypershiftContext() *HypershiftContext {
	hypershiftContextMu.Lock()
	defer hypershiftContextMu.Unlock()

	hostingNodeIPs := map[string]string{}
	for node, ip := range hypershiftContext.HostingNodesIPs {
		hostingNodeIPs[node] = ip
	}

	copyHypershiftContext := HypershiftContext{
		HostedClusters:  map[string]*HostedCluster{},
		HostingNodesIPs: hostingNodeIPs,
	}

	for clusterName, hostedCluster := range hypershiftContext.HostedClusters {
		controlPlanePorts := []int32{}
		for _, port := range hostedCluster.ControlPlanePorts {
			controlPlanePorts = append(controlPlanePorts, port)
		}

		copyHostedCluster := HostedCluster{
			ApiFQDN:           hostedCluster.ApiFQDN,
			ComputeNodeIP:     map[string]string{},
			ControlPlanePorts: controlPlanePorts,
			BaseDomain:        hostedCluster.BaseDomain,
		}

		for name, computeNode := range hostedCluster.ComputeNodeIP {
			copyHostedCluster.ComputeNodeIP[name] = computeNode
		}
		copyHypershiftContext.HostedClusters[clusterName] = &copyHostedCluster
	}

	return &copyHypershiftContext
}

func (a *HostedControlPlaneController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	hypershiftContextMu.Lock()
	defer hypershiftContextMu.Unlock()
	hostedControlPlane := &hypershift.HostedControlPlane{}
	err := a.Get(ctx, req.NamespacedName, hostedControlPlane)
	if err != nil {
		log.Error(err)
		return reconcile.Result{}, err
	}

	name := hostedControlPlane.Spec.InfraID
	hcp, present := hypershiftContext.HostedClusters[name]
	if !present {
		hcp = &HostedCluster{
			ComputeNodeIP: map[string]string{},
		}
		hypershiftContext.HostedClusters[name] = hcp
	}

	apiFQDN := hostedControlPlane.Status.ControlPlaneEndpoint.Host
	if len(apiFQDN) == 0 {
		log.Debugf("API for cluster %s not available", name)
		return reconcile.Result{}, nil
	}
	log.Infof("reconciling hosted control plane %s", apiFQDN)
	hcp.ApiFQDN = apiFQDN
	hcp.ControlPlanePorts = []int32{}

	serviceList := &corev1.ServiceList{}
	err = a.List(ctx, serviceList, client.InNamespace(hostedControlPlane.Namespace))
	if err != nil {
		log.Error(err)
		return reconcile.Result{}, err
	}

	for _, service := range serviceList.Items {
		for _, port := range service.Spec.Ports {
			if strings.Contains(strings.ToLower(service.Name), "ignition") {
				log.Infof("not adding ignition node port")
			} else {
				if port.NodePort > 0 {
					log.Infof("adding node port %d for service %s", port.NodePort, service.Name)
					hcp.ControlPlanePorts = append(hcp.ControlPlanePorts, port.NodePort)
				}
			}
		}
	}

	splits := strings.SplitAfter(apiFQDN, "api")

	if len(splits) >= 2 {
		hcp.BaseDomain = splits[1]
	}

	nodelist := &corev1.NodeList{}
	err = a.List(ctx, nodelist)
	if err != nil {
		log.Error(err)
		return reconcile.Result{}, err
	}
	for _, node := range nodelist.Items {
		for _, address := range node.Status.Addresses {
			if address.Type == corev1.NodeInternalIP || address.Type == corev1.NodeExternalIP {
				hypershiftContext.HostingNodesIPs[node.ObjectMeta.Name] = address.Address
				break
			}
		}
	}

	machineList := &capi.MachineList{}
	err = a.List(ctx, machineList)
	if err != nil {
		log.Error(err)
		return reconcile.Result{}, err
	}
	for _, machine := range machineList.Items {
		for _, address := range machine.Status.Addresses {
			if address.Type == capi.MachineInternalIP || address.Type == capi.MachineExternalIP {
				hcp.ComputeNodeIP[machine.ObjectMeta.Name] = address.Address
				break
			}
		}
	}
	return reconcile.Result{}, nil
}

func (a *MachineController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	hypershiftContextMu.Lock()
	defer hypershiftContextMu.Unlock()
	log.Info("reconciling machines")
	machine := &capi.Machine{}
	err := a.Get(ctx, req.NamespacedName, machine)
	if err != nil {
		log.Error(err)
		return reconcile.Result{}, err
	}

	labels := machine.ObjectMeta.Labels
	machineName := machine.ObjectMeta.Name
	name, present := labels[clusterNameLabel]
	if !present {
		log.Infof("machine %s does not have a cluster label", machineName)
		return reconcile.Result{}, nil
	}

	hcp, present := hypershiftContext.HostedClusters[name]
	if !present {
		log.Errorf("cluster associated with machine %s has not yet reconciled", machineName)
		return reconcile.Result{}, nil
	}
	for _, address := range machine.Status.Addresses {
		if address.Type == capi.MachineInternalIP || address.Type == capi.MachineExternalIP {
			hcp.ComputeNodeIP[machineName] = address.Address
			break
		}
	}
	return reconcile.Result{}, nil
}
