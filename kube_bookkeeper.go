package kube2consul

type KubeNode interface {
  name string;
  readyStatus bool;
  serviceIDS map[string]string;
}

type KubeBookkeeper interface {
  AddNode(string);
  RemoveNode(string) KubeNode;
  UpdateNodeReadyStatus(string, bool);
  GetNodes() []KubeNode
}


func (ks *kube2consul) removeService(obj interface{}) {
	if s, ok := obj.(*kapi.Service); ok {
		name := buildNameString(s.Name, s.Namespace)
		//Remove service from node
		for _, node := range ks.nodes {
			for _, id := range node.ids[name] {
				ks.removeDNS(id)
			}
			delete(node.ids, name)
		}
		delete(ks.services, name)
	}
}

func (ks *kube2consul) updateService(oldObj, newObj interface{}) {
	if old, ok := oldObj.(*kapi.Service); ok {
		if new, ok := newObj.(*kapi.Service); ok {
			if reflect.DeepEqual(*old, *new) == false {
				ks.removeService(old)
				ks.newService(new)
			}
		}
	}
}

func (ks *kube2consul) updateNode(oldObj, newObj interface{}) {
	if n, ok := newObj.(*kapi.Node); ok {
		ready := n.Status.Conditions[0].Status == kapi.ConditionTrue
		nodeInfo := ks.nodes[n.Name]

		if nodeInfo.ready != ready {
			glog.Infoln("Updating node:", n.Name, "with to ready status:", ready)
			nodeInfo.ready = ready
			if ready {
				for serviceName, service := range ks.services {
					ks.createDNS(serviceName, service, &nodeInfo)
				}
			} else {
				for _, serviceIDs := range nodeInfo.ids {
					for _, serviceID := range serviceIDs {
						ks.removeDNS(serviceID)
					}
				}
				nodeInfo.ids = make(map[string][]string) //Clear tha map
			}

			ks.nodes[n.Name] = nodeInfo
		}
	}
}

func (ks *kube2consul) newNode(newObj interface{}) {
	if node, ok := newObj.(*kapi.Node); ok {
		if _, ok := ks.nodes[node.Name]; !ok {
			glog.Info("Adding Node: ", node.Name)
			var newNode = *NewnodeInformation()
			newNode.name = node.Name
			newNode.address = node.Status.Addresses[0].Address

			ks.nodes[node.Name] = newNode
		}
	}
}

func (ks *kube2consul) removeNode(oldObj interface{}) {
	if node, ok := oldObj.(*kapi.Node); ok {
		if info, ok := ks.nodes[node.Name]; ok {
			glog.Info("Removing Node: ", node.Name)
			for _, idSet := range info.ids {
				for _, id := range idSet {
					ks.removeDNS(id)
				}
			}

			delete(ks.nodes, node.Name)
		}	else {
			glog.Infoln("Attempted to remove node ", node.Name, " not in inventory")
		}
	}
}
