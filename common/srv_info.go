package common

import (
	"time"
)

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
