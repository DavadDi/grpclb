package etcdv3

import (
	"log"
	"testing"
	"time"

	"github.com/DavadDi/grpclb/common"
)

func TestRegister(t *testing.T) {
	info := common.ServerNodeInfo{
		Name:           "greeter",
		Version:        "v1",
		Addr:           "127.0.0.1:8080",
		Weight:         1,
		LastUpdateTime: time.Now(),
	}

	r := EtcdRegister{
		EtcdAddrs:   []string{"127.0.0.1:2379"},
		DialTimeout: 3,
	}

	closeCh, err := r.Register(info, 10)
	if err != nil {
		t.Fatalf("Register to etcd failed." + err.Error())
	}

	infoEtcd, err := r.GetServiceInfo()
	if err != nil {
		t.Fatalf("Get from etcd failed.")
	}

	log.Printf("From etcd %#v\n", infoEtcd)
	time.Sleep(10 * time.Second)

	closeCh <- struct{}{}
}
