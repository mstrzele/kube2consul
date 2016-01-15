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
	"time"
	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kclientcmd "k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	kframework "k8s.io/kubernetes/pkg/controller/framework"
	kcache "k8s.io/kubernetes/pkg/client/cache"
	kSelector "k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/util"
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


/*
func Newkube2consul() *kube2consul {
	var k kube2consul
	k.nodes = make(map[string]nodeInformation)
	k.services = make(map[string]*kapi.Service)

	return &k
}
*/

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

	glog.Info("Parsed Master URL:", parsedUrl.String())
	return parsedUrl.String(), nil
}

//Take Service Object and Node
/*
func buildNameString(service *Service) string {
	//glog.Infof("Name String: %s  %s", service, namespace)
	//return fmt.Sprintf("%s.%s", service, namespace)
	return fmt.Sprintf("%s", service)
}
*/

func createKubeClient() (*kclient.Client, error) {
	masterUrl, err := getKubeMasterUrl()
	if err != nil {
		return nil, err
	}

	overrides := &kclientcmd.ConfigOverrides{}
	overrides.ClusterInfo.Server = masterUrl

	rules := kclientcmd.NewDefaultClientConfigLoadingRules()
	kubeConfig, err := kclientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()

	kubeConfig.Host = masterUrl
	if err != nil {
		glog.Error("Error creating Kube Config", err)
		return nil, err
	}
	glog.Infof("Using %s for kubernetes master", kubeConfig.Host)
	glog.Infof("Using kubernetes API %s", kubeConfig.Version)
	return kclient.New(kubeConfig)
}

// Returns a cache.ListWatch that gets all changes to services.
func createServiceLW(kubeClient *kclient.Client) *kcache.ListWatch {
	return kcache.NewListWatchFromClient(kubeClient, "services", kapi.NamespaceAll, kSelector.Everything())
}

// Returns a cache.ListWatch that gets all changes to nodes.
func createNodeLW(kubeClient *kclient.Client) *kcache.ListWatch {
	return kcache.NewListWatchFromClient(kubeClient, "nodes", kapi.NamespaceAll, kSelector.Everything())
}

func nodeReady(node *kapi.Node) bool {
	for  i := range node.Status.Conditions {
		if node.Status.Conditions[i].Type == kapi.NodeReady {
			return node.Status.Conditions[i].Status == kapi.ConditionTrue
		}
	}

	glog.Error("NodeReady condition is missing from node: ", node.Name)
	return false
}

//TODO Make Generic with Service
func sendNodeWork(action KubeWorkAction, queue chan<- KubeWork, oldObject,newObject interface{}) {
	if node, ok := newObject.(*kapi.Node); ok {
		glog.Info("Node Action: ", action, " for node ", node.Name)
		work := KubeWork {
			Action: action,
			Node: node,
		}

		if action == KubeWorkUpdateNode{
			if oldNode, ok := oldObject.(*kapi.Node); ok {
				if nodeReady(node) != nodeReady(oldNode) {
					glog.Info("Ready state change. Old:", nodeReady(oldNode), " New: ", nodeReady(node))
					queue <- work
				}
			}
		} else {
				queue <-work
		}
	}
}

//TODO Make Generic with Node
func sendServiceWork(action KubeWorkAction, queue chan<- KubeWork, serviceObj interface{}) {
	if service, ok := serviceObj.(*kapi.Service); ok {
		glog.Info("Service Action: ", action, " for service ", service.Name)
		queue <- KubeWork{
			Action: action,
			Service: service,
		}
	}
}

//TODO Combine with watchForServices
func watchForNodes(kubeClient *kclient.Client, queue chan<- KubeWork) {
	_, nodeController := kframework.NewInformer(
		createNodeLW(kubeClient),
		&kapi.Node{},
		resyncPeriod,
		kframework.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) {
				sendNodeWork(KubeWorkAddNode, queue, nil, obj)
			},
			DeleteFunc: func(obj interface{}) {
				sendNodeWork(KubeWorkRemoveNode, queue, nil, obj)
			},
			UpdateFunc: func(newObj,oldObj interface{}) {
				sendNodeWork(KubeWorkUpdateNode, queue, oldObj, newObj)
			},
		},
	)
	go nodeController.Run(util.NeverStop)
}

//TODO Combine with watchForNodes
func watchForServices(kubeClient *kclient.Client, queue chan<- KubeWork) {
	_, nodeController := kframework.NewInformer(
		createServiceLW(kubeClient),
		&kapi.Service{},
		resyncPeriod,
		kframework.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) {
				sendServiceWork(KubeWorkAddService, queue, obj)
			},
			DeleteFunc: func(obj interface{}) {
				sendServiceWork(KubeWorkRemoveService, queue, obj)
			},
			UpdateFunc: func(newObj,oldObj interface{}) {
				sendServiceWork(KubeWorkUpdateService, queue, newObj)
			},
		},
	)
	go nodeController.Run(util.NeverStop)
}

func main() {
	flag.Parse()

	//Attempt to create Kube Client
	kubeClient, err := createKubeClient()
	if err != nil {
		glog.Fatal("Could not connect to Kube Master", err)
	}

	if _, err := kubeClient.ServerVersion(); err != nil {
		glog.Fatal("Could not connect to Kube Master", err)
	} else {
		glog.Info("Connected to K8S API Server")
	}
	//Attempt to create Consul Client (All ways needed so that channels are not blocked)

	//Do System Setup stuff (create channels?)
	kubeWorkQueue := make(chan KubeWork)
	go RunBookKeeper(kubeWorkQueue)
	//Launch KubeLoops
	watchForNodes(kubeClient, kubeWorkQueue)
	watchForServices(kubeClient, kubeWorkQueue)
	//Launch Consul Loops

	//Launch Sync Loops (if needed)

	// Prevent exit
	select {}
}
