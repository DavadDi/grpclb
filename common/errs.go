package common

import (
	"errors"
)

// Errors ...
var (
	ErrNoEtcAddrs = errors.New("no etcd addrs provide")
)
