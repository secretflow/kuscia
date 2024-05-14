package coredns

import (
	"context"
	"time"

	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// Serial implements the Transferer interface.
func (e *KusciaCoreDNS) Serial(state request.Request) uint32 {
	return uint32(time.Now().Unix())
}

// MinTTL implements the Transferer interface.
func (e *KusciaCoreDNS) MinTTL(state request.Request) uint32 {
	return 30
}

// Transfer implements the Transferer interface.
func (e *KusciaCoreDNS) Transfer(ctx context.Context, state request.Request) (int, error) {
	return dns.RcodeServerFailure, nil
}
