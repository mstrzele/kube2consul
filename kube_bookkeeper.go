package main // import "github.com/jmccarty3/kube2consul"

import (
  "github.com/golang/glog"
  //"k8s.io/kubernetes/pkg/api"
  kapi "k8s.io/kubernetes/pkg/api"
)

//TODO: Chang to store Node pointer. Add getName, getReadyStatus accessors
type KubeNode struct {
  name string
  readyStatus bool;
  serviceIDS map[string]string;
}

type KubeBookKeeper interface {
  AddNode(*kapi.Node);
  RemoveNode(*kapi.Node);
  UpdateNode(*kapi.Node);
  SyncNodes();
  AddService(*kapi.Service)
  RemoveService(*kapi.Service)
  UpdateService(*kapi.Service)
}

type ClientBookKeeper struct {
  KubeBookKeeper
  nodes map[string]*KubeNode;
  services map[string]*kapi.Service
}

func BuildServiceBaseID(nodeName string, service *kapi.Service) string {
  return nodeName + "-" + service.Name
}

func NewKubeNode() *KubeNode {
  return &KubeNode {
    name: "",
    readyStatus: false,
    serviceIDS: make(map[string]string),
  }
}

func NewClientBookKeeper() *ClientBookKeeper {
  return &ClientBookKeeper {
    nodes: make(map[string]*KubeNode),
    services: make(map[string]*kapi.Service),
  }
}

func RunBookKeeper(workQue <-chan KubeWork) {

  client := NewClientBookKeeper()

  for work := range workQue {
    switch work.Action {
    case KubeWorkAddNode:
      client.AddNode(work.Node)
    case KubeWorkRemoveNode:
      client.RemoveNode(work.Node)
    case KubeWorkAddService:
      client.AddService(work.Service)
    case KubeWorkRemoveService:
      client.RemoveService(work.Service)
    case KubeWorkUpdateService:
      client.UpdateService(work.Service)
    default:
      glog.Info("Unsupported work action: ", work.Action)
    }
  }

  glog.Info("Completed all node work")
}

func (client *ClientBookKeeper) AttachServiceToNode(node *KubeNode, service *kapi.Service) {
  baseID := BuildServiceBaseID(node.name, service)
  //To Consol -> TODO
  glog.V(3).Info("Requesting Addition of services with Base ID: ", baseID)
  node.serviceIDS[service.Name] = baseID
}

func (client *ClientBookKeeper) DetachServiceFromNode(node *KubeNode, service *kapi.Service) {
  if baseID,ok := node.serviceIDS[service.Name]; ok {
    //To Consol -> TODO
    glog.V(3).Info("Requesting Removal of services with Base ID: ", baseID)
    delete(node.serviceIDS, service.Name)
  }
}

func (client *ClientBookKeeper) AddAllServicesToNode(node *KubeNode) {
  for _,service := range client.services {
    client.AttachServiceToNode(node, service)
  }
}

func (client *ClientBookKeeper) RemoveAllServicesFromNode(node *KubeNode) {
  for _,service := range client.services {
    client.DetachServiceFromNode(node,service)
  }
}

func (client *ClientBookKeeper) AddNode(newNode *kapi.Node) {
  if _, ok := client.nodes[newNode.Name]; ok {
    glog.Error("Attempted to Add existing node ", newNode.Name)
    return
  }

  //Add to Node Collection
  createdNode := NewKubeNode()
  createdNode.readyStatus = nodeReady(newNode)

  //Send request for Service Addition for node and all serviceIDS (Create Service ID here)
  if createdNode.readyStatus {
    client.AddAllServicesToNode(createdNode)
  }
  client.nodes[newNode.Name] = createdNode
  glog.Info("Added Node: ", newNode.Name)
}

func (client *ClientBookKeeper) RemoveNode(oldNode *kapi.Node) {
  if node,ok := client.nodes[oldNode.Name]; ok {
    //Remove All DNS for node
    client.RemoveAllServicesFromNode(node)
    //Remove Node from Collection
    delete(client.nodes, oldNode.Name)
  } else {
    glog.Error("Attmepted to remove missing node: ", oldNode.Name)
  }

}

func (client *ClientBookKeeper) UpdateNode(updatedNode *kapi.Node) {
  //If now ready -> Service Addtion for node
  //TODO Check it exists
  if nodeReady(updatedNode) {
    client.AddAllServicesToNode(client.nodes[updatedNode.Name])
  }
  //Else -> Service Removal for Node
  //UnLock
}

func SyncNodes() {
  //Get All Nodes from API
  //Lock Access
  //Add Remove as needed
  //UnLock
}


func (client *ClientBookKeeper) AddService(service *kapi.Service) {
  //TODO Verify it doesn't exist
  client.services[service.Name] = service
  //Perform All DNS Adds
  for _,node := range client.nodes {
    client.AttachServiceToNode(node, service)
  }

  glog.Info("Added Service: ", service.Name)
}

func  (client *ClientBookKeeper) RemoveService(service *kapi.Service) {
  //TODO Verify it does exist
  //Perform All DNS Removes
  for _,node := range client.nodes {
    client.DetachServiceFromNode(node, service)
  }

  delete(client.services, service.Name)
  glog.Info("Removed Service: ", service.Name)
}

func (client *ClientBookKeeper) UpdateService(service *kapi.Service) {
  client.RemoveService(service)
  client.AddService(service)
}
