# grpc_etcd_service_discovery

## 注册结构

```go
// ServerNodeInfo regiested to etcd
type ServerNodeInfo struct {
	Name           string    // 服务名称
	Version        string    // 服务版本号，用于服务升级过程中，配置兼容问题
	HostName       string    // 主机名称
	Addr           string    // 服务的地址, 格式为 ip:port，参见 https://github.com/grpc/grpc/blob/master/doc/naming.md
	Weight         uint16    // 服务权重
	LastUpdateTime time.Time // 更新时间使用租约机制
	MetaData       string    // 后续考虑替换成 interface{} 推荐 json 格式，服务端与客户端可以约定相关格式
}
```



## 服务注册

gRPC Server 启动的时候调用 Register 函数进行服务相关信息注册。注册过程需要使用 TLL 机制，保证服务信息因异常情况下失效。


## 服务访问
RW
gRPC 使用 LoadBanlance 中的 resover 使用注册的服务信息。首次连接的时候进行全量拉取，后续采用 Watcher 的方式来进行增删，为保证可靠性，设置定期拉取 Sync 的过程。

通过自定义 gRPC esover 的方式，保证对于 gRPC 服务端访问时的透明性。



# 测试

```
# 默认端口 2379
$ ./etcd

$ make test

$ ETCDCTL_API=3 ./etcdctl get --prefix=true ""
```
