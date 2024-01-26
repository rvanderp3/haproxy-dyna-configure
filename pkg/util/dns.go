package util

import (
	"fmt"
	"net"
)

func ResolveHost(hostname string) ([]net.IP, error) {
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return nil, fmt.Errorf("unable to resolve %s: %v", hostname, err)
	}
	return ips, nil
}
