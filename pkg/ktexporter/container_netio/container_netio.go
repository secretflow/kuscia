package container_netio

import (
	"fmt"
	"strconv"
	"strings"

	"os"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

func GetContainerNetIOFromProc(defaultIface, pid string) (recvBytes, xmitBytes uint64, err error) {
	netDevPath := fmt.Sprintf("/proc/%s/net/dev", pid)
	data, err := os.ReadFile(netDevPath)
	if err != nil {
		nlog.Warn("Fail to read the path", netDevPath)
		return recvBytes, xmitBytes, err
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) < 3 {
		nlog.Error("unexpected format in ", netDevPath)
		return recvBytes, xmitBytes, err
	}
	recvByteStr := ""
	xmitByteStr := ""
	for _, line := range lines {
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}

		iface := strings.Trim(fields[0], ":")
		if iface == defaultIface {
			recvByteStr = fields[1]
			xmitByteStr = fields[9]
		}
	}
	if recvByteStr == "" {
		recvByteStr = "0"
	}
	if xmitByteStr == "" {
		xmitByteStr = "0"
	}
	recvBytes, err = strconv.ParseUint(recvByteStr, 10, 64)
	if err != nil {
		nlog.Error("Error converting string to uint64:", err)
		return recvBytes, xmitBytes, err
	}
	xmitBytes, err = strconv.ParseUint(xmitByteStr, 10, 64)
	if err != nil {
		nlog.Error("Error converting string to uint64:", err)
		return recvBytes, xmitBytes, err
	}

	return recvBytes, xmitBytes, nil
}

func GetContainerBandwidth(curRecvBytes, preRecvBytes, curXmitBytes, preXmitBytes uint64, timeWindow float64) (recvBandwidth, xmitBandwidth float64, err error) {
	recvBytesDiff := float64(curRecvBytes) - float64(preRecvBytes)
	xmitBytesDiff := float64(curXmitBytes) - float64(preXmitBytes)

	recvBandwidth = (recvBytesDiff * 8) / timeWindow
	xmitBandwidth = (xmitBytesDiff * 8) / timeWindow

	return recvBandwidth, xmitBandwidth, nil
}
