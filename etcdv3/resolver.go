package etcdv3

import (
	"context"
	"log"
	"time"

	"github.com/DavadDi/grpclb/common"
	"github.com/DavadDi/grpclb/utils"
	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/mvcc/mvccpb"
	"google.golang.org/grpc/resolver"
)

// Default shchme
const (
	etcdScheme = "etcd"
)

// NewEtcdResolver return a new grpc Resolver based on etcd
func NewEtcdResolver(etcdAddrs []string, srvName, srvVersion string, ttl int64) *EtcdResolver {
	return &EtcdResolver{
		scheme: etcdScheme,

		EtcdAddrs:   etcdAddrs,
		DialTimeout: 3,

		SrvName:    srvName,
		SrvVersion: srvVersion,
		SrvTTL:     ttl,
	}
}

// EtcdResolver for grpc client loadbalance From "google.golang.org/grpc/resolver/manual"
type EtcdResolver struct {
	bootstrapAddrs []resolver.Address // 初始化的地址，可以为空
	scheme         string             // Resolver scheme 默认为 etcd

	EtcdAddrs   []string // Etcd 集群地址
	DialTimeout int      // Etcd 集群连接超时时间

	SrvName    string // 服务名称
	SrvVersion string // 服务版本
	SrvTTL     int64  // 服务 TLL 时间，默认为秒

	closeCh      chan struct{}      // 关闭 channel
	cli          *clientv3.Client   // etcd client
	watchCh      clientv3.WatchChan // watch channel
	keyPrifix    string             // watch key prefix
	srvAddrsList []resolver.Address // watch server addrs list

	cc resolver.ClientConn

	usedForTest bool // used only for test resover, if no grpc dial regiter client.Conn callback
}

// Build returns itself for resolver, because it's both a builder and a resolver.
func (r *EtcdResolver) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOption) (resolver.Resolver, error) {
	r.cc = cc
	_, err := r.start()
	if err != nil {
		return nil, err
	}

	return r, nil
}

// Scheme returns the scheme.
func (r *EtcdResolver) Scheme() string {
	return r.scheme
}

// ResolveNow is a noop for resolver.
func (r *EtcdResolver) ResolveNow(o resolver.ResolveNowOption) {}

// Close is a noop for resolver.
func (r *EtcdResolver) Close() {
	log.Println("EtcdResolver Close()")
	r.closeCh <- struct{}{}
}

// NewAddress to update cc
func (r *EtcdResolver) NewAddress(addrs []resolver.Address) {
	r.cc.NewAddress(addrs)
}

// Start Resover return a closeCh, Should call by Builde func()
func (r *EtcdResolver) start() (chan<- struct{}, error) {
	r.scheme = etcdScheme

	// TODO check etcd addrs
	var err error

	r.cli, err = clientv3.New(clientv3.Config{
		Endpoints:   r.EtcdAddrs,
		DialTimeout: time.Duration(r.DialTimeout) * time.Second,
	})

	if err != nil {
		return nil, err
	}

	if len(r.bootstrapAddrs) > 0 {
		r.NewAddress(r.bootstrapAddrs)
	}

	resolver.Register(r)

	r.keyPrifix = utils.BuildPrefix(common.ServerNodeInfo{Name: r.SrvName, Version: r.SrvVersion})
	r.closeCh = make(chan struct{})

	err = r.sync()
	if err != nil {
		return nil, err
	}

	go r.watch()

	return r.closeCh, nil
}

// Print just for usedForTest
func (r *EtcdResolver) Print(name string) {
	log.Printf("-------- %s --------\n", name)
	log.Printf("Scheme: %s\n", r.scheme)
	log.Printf("EtcdAddres: %v\n", r.EtcdAddrs)
	log.Printf("SrvName: %s\n", r.SrvName)
	log.Printf("SrvVersion: %s\n", r.SrvVersion)
	log.Printf("SrvTTL: %d\n", r.SrvTTL)
	log.Printf("KeyPrifix： %s\n", r.keyPrifix)
	log.Printf("srvAddrsList： %v\n", r.srvAddrsList)
	log.Printf("-------------------\n")
}

// sync full addrs
func (r *EtcdResolver) sync() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	getResp, err := r.cli.Get(ctx, r.keyPrifix, clientv3.WithPrefix())
	if err != nil {
		log.Println(err)
		return err
	}

	// r.srvAddrsList = r.srvAddrsList[:0] This might be the optimal approach in some scenarios.
	// But it might also be a cause of "memory leaks" - memory not used, but potentially reachable
	// (after re-slicing of 'slice') and thus not garbage "collectable".
	// see https://codeday.me/bug/20170712/41862.html
	r.srvAddrsList = []resolver.Address{}
	for i := range getResp.Kvs {
		addr := resolver.Address{Addr: utils.SplitPath(string(getResp.Kvs[i].Key), r.keyPrifix)}
		r.srvAddrsList = append(r.srvAddrsList, addr)
	}

	if !r.usedForTest {
		r.NewAddress(r.srvAddrsList)
	}

	// r.Print(" Sync ")
	return nil
}

// update ...
func (r *EtcdResolver) update(evs []*clientv3.Event) {
	for _, ev := range evs {
		key := string(ev.Kv.Key)
		addr := resolver.Address{Addr: utils.SplitPath(key, r.keyPrifix)}
		switch ev.Type {
		case mvccpb.PUT:
			if !exist(r.srvAddrsList, addr) {
				r.srvAddrsList = append(r.srvAddrsList, addr)
				if !r.usedForTest {
					r.NewAddress(r.srvAddrsList)
				}
			}
		case mvccpb.DELETE:
			if s, ok := remove(r.srvAddrsList, addr); ok {
				r.srvAddrsList = s
				if !r.usedForTest {
					r.NewAddress(r.srvAddrsList)
				}
			}

		}
	}

	// r.Print(" Update ")
}

// watch addrs update event
func (r *EtcdResolver) watch() {
	// 全量同步信息的时间间隔设置为 TTL 的 5 倍
	ticker := time.NewTicker(time.Second * time.Duration(r.SrvTTL*5))
	r.watchCh = r.cli.Watch(context.Background(), r.keyPrifix, clientv3.WithPrefix())

	for {
		select {
		case <-r.closeCh:
			return

		case ev, ok := <-r.watchCh:
			if ok {
				r.update(ev.Events)
			}
		case <-ticker.C:
			// timeout, full sync
			err := r.sync()
			if err != nil {
				log.Printf("Time Sync failed. %s\n", err.Error())
			}
		}
	}
}

// helper function
func exist(l []resolver.Address, addr resolver.Address) bool {
	for i := range l {
		if l[i].Addr == addr.Addr {
			return true
		}
	}
	return false
}

// helper function
func remove(s []resolver.Address, addr resolver.Address) ([]resolver.Address, bool) {
	for i := range s {
		if s[i].Addr == addr.Addr {
			s[i] = s[len(s)-1]
			return s[:len(s)-1], true
		}
	}
	return nil, false
}
