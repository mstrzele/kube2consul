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

	//"github.com/google/cadvisor/info/v1"
	//"github.com/imdario/mergo"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kclientcmd "k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
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

	glog.Info("Overrids server:", overrides.ClusterInfo.Server)

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

	//Launch KubeLoops
	//Launch Consul Loops

	//Launch Sync Loops (if needed)

	// Prevent exit
	select {}
}
