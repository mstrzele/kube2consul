package kube2consul

import(
  "strconv"
	"strings"

  "github.com/golang/glog"
	consulapi "github.com/hashicorp/consul/api"
	kapi "k8s.io/kubernetes/pkg/api"
)

type ConsulWorker interface {
  AddDNS()()
  RemoveDNS()()
  GetServices()()
}

type ConsulClient struct {
  ConsulWorker
}


func (ks *kube2consul) createDNS(record string, service *kapi.Service, node *nodeInformation) error {
	if strings.Contains(record, ".") {
		glog.Infof("Service names containing '.' are not supported: %s\n", service.Name)
		return nil
	}

	// if ClusterIP is not set, do not create a DNS records
	if !kapi.IsServiceIPSet(service) {
		glog.Infof("Skipping dns record for headless service: %s\n", service.Name)
		return nil
	}

	//Currently this is only for NodePorts.
	if service.Spec.Type != kapi.ServiceTypeNodePort {
		glog.V(3).Infof("Skipping non-NodePort service: %s\n", service.Name)
		return nil
	}

	for i := range service.Spec.Ports {
		newId := node.name + record + service.Spec.Ports[i].Name
		var asrName string

		//If the port has a name. Use that.
		if len(service.Spec.Ports[i].Name) > 0 {
			asrName = record + "-" + service.Spec.Ports[i].Name
		} else if len(service.Spec.Ports) == 1 { //TODO: Pull out logic later
			asrName = record
		} else {
			asrName = record + "-" + strconv.Itoa(service.Spec.Ports[i].Port)
		}

		if Contains(node.ids[record], newId) == false {
			asr := &consulapi.AgentServiceRegistration{
				ID:      newId,
				Name:    asrName,
				Address: node.address,
				Port:    service.Spec.Ports[i].NodePort,
				Tags:    []string{"Kube", string(service.Spec.Ports[i].Protocol) },
			}

			if *argChecks && service.Spec.Ports[i].Protocol == "TCP" {
				target := string(node.address) + ":" + strconv.Itoa(service.Spec.Ports[i].NodePort)
				glog.Infof("Created Service check for: %v on %v\n", asr.Name, target)
				asr.Check = &consulapi.AgentServiceCheck {
					TCP: target,
					Interval: "60s",
				}
			}

			glog.Infof("Setting DNS record: %v -> %v:%d with protocol %v\n", asr.Name, asr.Address, asr.Port, string(service.Spec.Ports[i].Protocol))

			if ks.consulClient != nil {
				if err := ks.consulClient.Agent().ServiceRegister(asr); err != nil {
					glog.Errorf("Error registering with consul: %v\n", err)
					return err
				}
			}

			node.ids[record] = append(node.ids[record], newId)
		}
	}
	return nil
}

func (ks *kube2consul) removeDNS(recordID string) error {
	glog.Infof("Removing %s from DNS", recordID)
	if ks.consulClient != nil {
		ks.consulClient.Agent().ServiceDeregister(recordID)
	}
	return nil
}

func newConsulClient(consulAgent string) (*consulapi.Client, error) {
	var (
		client *consulapi.Client
		err    error
	)

	consulConfig := consulapi.DefaultConfig()
	consulAgentUrl, err := url.Parse(consulAgent)
	if err != nil {
		glog.Infof("Error parsing Consul url")
		return nil, err
	}

	if consulAgentUrl.Host != "" {
		consulConfig.Address = consulAgentUrl.Host
	}

	if consulAgentUrl.Scheme != "" {
		consulConfig.Scheme = consulAgentUrl.Scheme
	}

	client, err = consulapi.NewClient(consulConfig)
	if err != nil {
		glog.Infof("Error creating Consul client")
		return nil, err
	}

	for attempt := 1; attempt <= maxConnectAttempts; attempt++ {
		if _, err = client.Agent().Self(); err == nil {
			break
		}

		if attempt == maxConnectAttempts {
			break
		}

		glog.Infof("[Attempt: %d] Attempting access to Consul after 5 second sleep", attempt)
		time.Sleep(5 * time.Second)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to Consul agent: %v, error: %v", consulAgent, err)
	}
	glog.Infof("Consul agent found: %v", consulAgent)

	return client, nil
}
