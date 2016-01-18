package main // import "github.com/jmccarty3/kube2consul"

import (
  //"k8s.io/kubernetes/pkg/api"
  kapi "k8s.io/kubernetes/pkg/api"
)

type KubeWorkAction string

const (
  KubeWorkAddNode KubeWorkAction = "AddNode"
  KubeWorkRemoveNode KubeWorkAction = "RemoveNode"
  KubeWorkUpdateNode KubeWorkAction = "UpdateNode"
  KubeWorkAddService KubeWorkAction = "AddService"
  KubeWorkRemoveService KubeWorkAction = "RemoveService"
  KubeWorkUpdateService KubeWorkAction = "UpdateService"
  KubeWorkSync KubeWorkAction = "Sync"
)

//TODO: Consider just taking the api.Node Object
type KubeWork struct {
  Action KubeWorkAction
  Node *kapi.Node
  Service *kapi.Service
}

type ConsulWorkAction string

const (
  ConsulWorkAddDNS ConsulWorkAction = "AddDNS"
  ConsulWorkRemoveDNS ConsulWorkAction = "RemoveDNS"
  ConsulWorkSyncDNS ConsulWorkAction = "SyncDNS"
)

type DnsInfo struct {
  BaseID string
  IPAddress string
}

type ConsulWork struct {
  Action ConsulWorkAction
  Service *kapi.Service
  Config DnsInfo
}
