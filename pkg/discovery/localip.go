package discovery

import (
	"fmt"
	"net"
	"strings"
)

// return this machine IPv4 addr by scanning physical network interfaces
func GetLocalIP() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("failed to list network interfaces: %w", err)
	}

	for _, iface := range interfaces {
		// Skip down interfaces and loopback
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		// Ignore virtual adapters (WSL, Docker, Hyper-V, VirtualBox)
		name := strings.ToLower(iface.Name)
		if strings.Contains(name, "vethernet") || strings.Contains(name, "docker") ||
			strings.Contains(name, "vbox") || strings.Contains(name, "wsl") {
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
			if ip4 := ipNet.IP.To4(); ip4 != nil {
				return ip4.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no active LAN IPv4 address found")
}

// return this machine IPv4 addr by scanning network
// func GetLocalIP() (string, error) {
// 	addrs, err := net.InterfaceAddrs()
// 	if err != nil {
// 		return "", fmt.Errorf("failed to list network interfaces: %w", err)
// 	}

// 	for _, addr := range addrs {
// 		ipNet, ok := addr.(*net.IPNet)
// 		if !ok || ipNet.IP.IsLoopback() {
// 			continue
// 		}
// 		if ip4 := ipNet.IP.To4(); ip4 != nil {
// 			return ip4.String(), nil
// 		}
// 	}

// 	return "", fmt.Errorf("no LAN IPv4 address found, make sure you connnected to internet")
// }
