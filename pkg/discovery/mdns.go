package discovery

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/grandcat/zeroconf"
)

// defines the mDNS service domain name for BeamIt nodes
const ServiceType = "_beamit._tcp"

// holds metadata about a discovered peer on the local network
type PeerInfo struct {
	ID       string
	Hostname string
	IP       string
	Port     int
	FileName string
}

// broadcast this sender device on the local network via mDNS
func AnnouncePeer(port int, fileName string) (*zeroconf.Server, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown-device"
	}

	// TXT records allow us to attach small key-value metadata to the broadcast
	meta := []string{
		fmt.Sprintf("filename=%s", fileName),
		fmt.Sprintf("version=0.1.0"),
	}

	// registes mDNS service on local network
	server, err := zeroconf.Register(
		hostname,    // service instance name (e.g., "john_doe-latop)
		ServiceType, // service type
		"local.",    // domain
		port,        // port where TransferServer is listening
		meta,        // text metadata
		nil,         // network interfaces (nil == all)
	)
	if err != nil {
		return nil, fmt.Errorf("failed to register mDNS service: %w", err)
	}

	return server, nil
}

// scans the local Wi-Fi network for active BeamIt senders
func DiscoverPeers(timeout time.Duration) ([]PeerInfo, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize mDNS resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry)
	var discovered []PeerInfo

	// gorotine. for collecting entries as they are found
	go func() {
		for entry := range entries {
			if len(entry.AddrIPv4) == 0 {
				continue
			}

			// extract filename metadata from TXT records
			fileName := "Unknown File"
			for _, text := range entry.Text {
				var fn string
				if _, err := fmt.Sscanf(text, "filename=%s", &fn); err == nil {
					fileName = fn
				}
			}

			peer := PeerInfo{
				ID:       entry.Instance,
				Hostname: entry.HostName,
				IP:       entry.AddrIPv4[0].String(),
				Port:     entry.Port,
				FileName: fileName,
			}
			discovered = append(discovered, peer)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// browse for active BeamIt services on local network
	err = resolver.Browse(ctx, ServiceType, "local.", entries)
	if err != nil {
		return nil, fmt.Errorf("failed browsing local network: %w", err)
	}

	<-ctx.Done() // wait until timeout finishes
	return discovered, nil
}
