package discovery

import (
	"fmt"
	"net"
	"strings"
)

var virtualAdapterMarkers = []string{"vethernet", "docker", "vbox", "wsl"}

// return this machine IPv4 addr by scanning physical network interfaces
func GetLocalIP() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("failed to list network interfaces: %w", err)
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		name := strings.ToLower(iface.Name)
		skip := false
		for _, marker := range virtualAdapterMarkers {
			if strings.Contains(name, marker) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP.IsLoopback() {
				continue
			}
			ip4 := ipNet.IP.To4()
			if ip4 == nil || ip4.IsLinkLocalUnicast() {
				continue // skip 169.254.x.x APIPA addresses from unconfigured adapters
			}
			return ip4.String(), nil
		}
	}

	return "", fmt.Errorf("no active LAN IPv4 address found")
}
