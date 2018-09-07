package etcdv3

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/DavadDi/grpclb/common"
	"github.com/DavadDi/grpclb/utils"
	"go.etcd.io/etcd/clientv3"
)

// NewEtcdRegister return a EtcdRegister, param: etcd endpoints addrs. Just for simple use
func NewEtcdRegister(etcdAddrs []string) *EtcdRegister {
	return &EtcdRegister{
		EtcdAddrs:   etcdAddrs,
		DialTimeout: 3,
	}
}

// EtcdRegister ...
type EtcdRegister struct {
	EtcdAddrs   []string
	DialTimeout int

	// server
	srvInfo     common.ServerNodeInfo
	srvTTL      int64
	cli         *clientv3.Client
	leasesID    clientv3.LeaseID
	keepAliveCh <-chan *clientv3.LeaseKeepAliveResponse
	closeCh     chan struct{}
}

// Register a service base on ServerNodeInfo。
func (r *EtcdRegister) Register(srvInfo common.ServerNodeInfo, ttl int64) (chan<- struct{}, error) {
	// check etcd addrs
	if len(r.EtcdAddrs) <= 0 {
		return nil, common.ErrNoEtcAddrs
	}

	var err error
	r.cli, err = clientv3.New(clientv3.Config{
		Endpoints:   r.EtcdAddrs,
		DialTimeout: time.Duration(r.DialTimeout) * time.Second,
	})

	if err != nil {
		return nil, err
	}

	r.srvInfo = srvInfo
	r.srvTTL = ttl

	leaseCtx, cancel := context.WithTimeout(context.Background(), time.Duration(r.DialTimeout)*time.Second)
	leaseResp, err := r.cli.Grant(leaseCtx, ttl)
	cancel()
	if err != nil {
		return nil, err
	}

	r.leasesID = leaseResp.ID
	r.keepAliveCh, err = r.cli.KeepAlive(context.Background(), leaseResp.ID)
	if err != nil {
		return nil, err
	}

	r.closeCh = make(chan struct{})

	// Registe the svc At FIRST TIME
	err = r.register()
	if err != nil {
		return nil, err
	}

	go r.keepAlive()

	return r.closeCh, nil
}

// Stop registe process
func (r *EtcdRegister) Stop() {
	r.closeCh <- struct{}{}
}

// GetServiceInfo used get service info from etcd. Used for TEST
func (r *EtcdRegister) GetServiceInfo() (common.ServerNodeInfo, error) {
	resp, err := r.cli.Get(context.Background(), utils.BuildRegPath(r.srvInfo))
	if err != nil {
		return r.srvInfo, err
	}

	infoRes := common.ServerNodeInfo{}
	for idx, val := range resp.Kvs {
		log.Printf("[%d] %s %s\n", idx, string(val.Key), string(val.Value))
	}

	// only return one recorde
	if resp.Count >= 1 {
		err = json.Unmarshal(resp.Kvs[0].Value, &infoRes)
		if err != nil {
			return infoRes, err
		}
	}

	return infoRes, nil
}

// register service into to etcd
func (r *EtcdRegister) register() error {
	r.srvInfo.LastUpdateTime = time.Now()
	regData, err := json.Marshal(r.srvInfo)
	if err != nil {
		return err
	}

	_, err = r.cli.Put(context.Background(), utils.BuildRegPath(r.srvInfo), string(regData), clientv3.WithLease(r.leasesID))

	return err
}

// unregister service from etcd
func (r *EtcdRegister) unregister() error {
	_, err := r.cli.Delete(context.Background(), utils.BuildRegPath(r.srvInfo))

	return err
}

// keepAlive ...
func (r *EtcdRegister) keepAlive() {
	// 后续考虑将更新时间设定某个范围的浮动
	ticker := time.NewTicker(time.Second * time.Duration(r.srvTTL))
	for {
		select {
		case <-r.closeCh:
			// log.Printf("unregister %s\n", r.srvInfo.Addr)
			err := r.unregister()
			if err != nil {
				log.Printf("unregister %s failed. %s\n", r.srvInfo.Addr, err.Error())
			}
			// clean lease
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_, err = r.cli.Revoke(ctx, r.leasesID)
			if err != nil {
				log.Printf("Revoke lease %d for %s failed. %s\n", r.leasesID, r.srvInfo.Addr, err.Error())
			}

			return
		case <-r.keepAliveCh:
			// Just do nothing, closeCh should revoke lease
			// log.Printf("recv keepalive %s, %d\n", r.srvInfo.Addr, len(r.keepAliveCh))
		case <-ticker.C:
			// Timeout, no check out and just put
			// log.Printf("register %s\n", r.srvInfo.Addr)
			err := r.register()
			if err != nil {
				log.Printf("register %s failed. %s\n", r.srvInfo.Addr, err.Error())
			}

		}
	}
}
