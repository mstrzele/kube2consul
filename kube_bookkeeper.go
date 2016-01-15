package main // import "github.com/jmccarty3/kube2consul"

import (
  //"k8s.io/kubernetes/pkg/api"
)

type KubeNode struct {
  name string;
  readyStatus bool;
  serviceIDS map[string]string;
}

type KubeBookKeeper interface {
  AddNode(string, bool);
  RemoveNode(string);
  UpdateNode(string, bool);
  SyncNodes();
}


func SetupBookKeeper() {

}

func AddNode(name string, ready bool) {
  //Lock Node Access
  //Add to Node Collection
  //Send request for Service Addition for node and all serviceIDS (Create Service ID here)
  //Unlock node access
}

func RemoveNode(name string) {
  //Lock Node Access
  //Remove Node from Collection
  //Remove All DNS for node
  //UnLock
}

func UpdateNode(name string, ready bool) {
  //Lock Node
  //If now ready -> Service Addtion for node
  //Else -> Service Removal for Node
  //UnLock
}

func SyncNodes() {
  //Get All Nodes from API
  //Lock Access
  //Add Remove as needed
  //UnLock
}

/*
func AddService(Service) {
  //Lock Services
  //Add Service
  //Lock Nodes
  //Perform All DNS Adds
  //Unlock Nodes
  //Unlock Services
}

func RemoveService(Service) {
  //Lock Services
  //Remove Service
  //Lock Nodes
  //Perform All DNS Removes
  //Unlock Nodes
  //Unlock Services
}

func UpdateService(Service) {
  RemoveService(Service)
  AddService(Service)
}
*/
