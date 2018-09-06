package utils

import (
	"fmt"
	"github.com/DavadDi/grpclb/common"
	"strings"
)

// BuildPrefix has last "/"
func BuildPrefix(info common.ServerNodeInfo) string {
	return fmt.Sprintf("/%s/%s/", info.Name, info.Version)
}
func BuildRegPath(info common.ServerNodeInfo) string {
	return fmt.Sprintf("%s%s",
		BuildPrefix(info), info.Addr)
}

// split addr from reg full path ip:port
func SplitPath(path, prefix string) string {
	return strings.TrimPrefix(path, prefix)
}
