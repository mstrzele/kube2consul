/*
Copyright 2014 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// kube2consul is a bridge between Kubernetes and Consul.  It watches the
// Kubernetes master for changes in Services and creates new DNS records on the
// consul agent.

package main // import "github.com/jmccarty3/kube2consul"

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	consulapi "github.com/hashicorp/consul/api"
	kapi "k8s.io/kubernetes/pkg/api"
	kcache "k8s.io/kubernetes/pkg/client/cache"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kclientcmd "k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	kcontrollerFramework "k8s.io/kubernetes/pkg/controller/framework"
	kframework "k8s.io/kubernetes/pkg/controller/framework"
	kSelector "k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/util"

	//  "k8s.io/kubernetes/pkg/api"
	//  "k8s.io/kubernetes/pkg/fields"
	//  "k8s.io/kubernetes/pkg/labels"
)

var (
	argConsulAgent   = flag.String("consul-agent", "http://127.0.0.1:8500", "URL to consul agent")
	argKubecfgFile   = flag.String("kubecfg_file", "", "Location of kubecfg file for access to kubernetes service")
	argKubeMasterUrl = flag.String("kube_master_url", "https://${KUBERNETES_SERVICE_HOST}:${KUBERNETES_SERVICE_PORT}", "Url to reach kubernetes master. Env variables in this flag will be expanded.")
	argDryRun        = flag.Bool("dryrun", false, "Runs without connecting to consul")
	argChecks        = flag.Bool("checks", false, "Adds TCP service checks for each TCP Service")
)

const (
	// Maximum number of attempts to connect to consul server.
	maxConnectAttempts = 12
	// Resync period for the kube controller loop.
	resyncPeriod = 5 * time.Second
)

type nodeInformation struct {
	name string

	address string

	ready bool
	// map[service] DNS IDs
	ids map[string][]string
}

type kube2consul struct {
	// Consul client.
	consulClient *consulapi.Client

	// DNS domain name.
	domain string

	//Nodes Name / valid
	nodes map[string]nodeInformation

	//All Services.
	services map[string]*kapi.Service
}

func Newkube2consul() *kube2consul {
	var k kube2consul
	k.nodes = make(map[string]nodeInformation)
	k.services = make(map[string]*kapi.Service)

	return &k
}

func NewnodeInformation() *nodeInformation {
	var n nodeInformation
	n.ready = false
	n.ids = make(map[string][]string)

	return &n
}

func Contains(s []string, e string) bool {
	for _, i := range s {
		if i == e {
			return true
		}
	}
	return false
}

func getKubeMasterUrl() (string, error) {
	if *argKubeMasterUrl == "" {
		return "", fmt.Errorf("no --kube_master_url specified")
	}
	parsedUrl, err := url.Parse(os.ExpandEnv(*argKubeMasterUrl))
	if err != nil {
		return "", fmt.Errorf("failed to parse --kube_master_url %s - %v", *argKubeMasterUrl, err)
	}
	if parsedUrl.Scheme == "" || parsedUrl.Host == "" || parsedUrl.Host == ":" {
		return "", fmt.Errorf("invalid --kube_master_url specified %s", *argKubeMasterUrl)
	}
	return parsedUrl.String(), nil
}

// TODO: evaluate using pkg/client/clientcmd
func newKubeClient() (*kclient.Client, error) {
	var config *kclient.Config
	masterUrl, err := getKubeMasterUrl()
	if err != nil {
		return nil, err
	}
	if *argKubecfgFile == "" {
		config = &kclient.Config{
			Host:    masterUrl,
			Version: "v1",
		}
	} else {
		var err error
		overrides := &kclientcmd.ConfigOverrides{}
		overrides.ClusterInfo.Server = masterUrl                                     // might be "", but that is OK
		rules := &kclientcmd.ClientConfigLoadingRules{ExplicitPath: *argKubecfgFile} // might be "", but that is OK
		if config, err = kclientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig(); err != nil {
			return nil, err
		}
	}
	glog.Infof("Using %s for kubernetes master", config.Host)
	glog.Infof("Using kubernetes API %s", config.Version)
	return kclient.New(config)
}

func buildNameString(service, namespace string) string {
	//glog.Infof("Name String: %s  %s", service, namespace)
	//return fmt.Sprintf("%s.%s", service, namespace)
	return fmt.Sprintf("%s", service)
}

// Returns a cache.ListWatch that gets all changes to services.
func createServiceLW(kubeClient *kclient.Client) *kcache.ListWatch {
	return kcache.NewListWatchFromClient(kubeClient, "services", kapi.NamespaceAll, kSelector.Everything())
}

// Returns a cache.ListWatch that gets all changes to services.
func createNodeLW(kubeClient *kclient.Client) *kcache.ListWatch {
	return kcache.NewListWatchFromClient(kubeClient, "nodes", kapi.NamespaceAll, kSelector.Everything())
}

func (ks *kube2consul) newService(obj interface{}) {
	if s, ok := obj.(*kapi.Service); ok {
		name := buildNameString(s.Name, s.Namespace)
		ks.services[name] = s

		glog.V(2).Info("Creating Service: ", name)
		//Add to all existing nodes
		for _, node := range ks.nodes {
			if node.ready {
				ks.createDNS(name, s, &node)
			}
		}
	}
}



func watchForServices(kubeClient *kclient.Client, ks *kube2consul) {
	var serviceController *kcontrollerFramework.Controller
	_, serviceController = kframework.NewInformer(
		createServiceLW(kubeClient),
		&kapi.Service{},
		resyncPeriod,
		kframework.ResourceEventHandlerFuncs{
			AddFunc:    ks.newService,
			DeleteFunc: ks.removeService,
			UpdateFunc: ks.updateService,
		},
	)
	go serviceController.Run(util.NeverStop)
}

func watchForNodes(kubeClient *kclient.Client, ks *kube2consul) kcache.Store {
	store, serviceController := kframework.NewInformer(
		createNodeLW(kubeClient),
		&kapi.Node{},
		resyncPeriod,
		kframework.ResourceEventHandlerFuncs{
			AddFunc:    ks.newNode,
			DeleteFunc: ks.removeNode,
			UpdateFunc: ks.updateNode,
		},
	)
	glog.Info("About to call run!")
	go serviceController.Run(util.NeverStop)
	return store
}

func main() {
	flag.Parse()
	var err error
	// TODO: Validate input flags.
	ks := Newkube2consul()

	if *argDryRun {
		glog.Info("Dryrun started. Ignoring Consul")
	} else {
		if ks.consulClient, err = newConsulClient(*argConsulAgent); err != nil {
			glog.Fatalf("Failed to create Consul client - %v", err)
		}
	}

	kubeClient, err := newKubeClient()
	if err != nil {
		glog.Fatalf("Failed to create a kubernetes client: %v", err)
	}

	glog.Info(kubeClient.ServerVersion())

	watchForServices(kubeClient, ks)
	watchForNodes(kubeClient, ks)
	glog.Info("Watchers running")
	select {}
}
