package pkg

import (
	"github.com/threefoldtech/test/pkg/gridtypes/test"
)

//go:generate mkdir -p stubs

//go:generate zbusc -module qsfsd -version 0.0.1 -name manager -package stubs github.com/threefoldtech/test/pkg+QSFSD stubs/qsfsd_stub.go

type QSFSMetrics struct {
	Consumption map[string]NetMetric
}

type QSFSInfo struct {
	Path            string
	MetricsEndpoint string
}

func (q *QSFSMetrics) Nu(wlID string) (result uint64) {
	if v, ok := q.Consumption[wlID]; ok {
		result += v.NetRxBytes
		result += v.NetTxBytes
	}
	return
}

type QSFSD interface {
	Mount(wlID string, cfg test.QuantumSafeFS) (QSFSInfo, error)
	Unmount(wlID string) error
	Metrics() (QSFSMetrics, error)
}
