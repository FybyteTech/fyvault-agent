//go:build linux

package ebpf

import (
	"fmt"
	"net"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"go.uber.org/zap"
)

// Generate Go bindings from eBPF C code at build time.
// This would normally use: //go:generate go run github.com/cilium/ebpf/cmd/bpf2go ...
// For now, we load from a pre-compiled .o file.

// TargetKey mirrors struct target_key in the eBPF C program.
type TargetKey struct {
	DstIP   uint32 // network byte order
	DstPort uint16 // network byte order
	Pad     uint16
}

// TargetValue mirrors struct target_value in the eBPF C program.
type TargetValue struct {
	ProxyPort uint16 // network byte order
	Pad       uint16
}

// Program holds a loaded eBPF TC classifier and its maps.
type Program struct {
	iface     string
	targetMap *ebpf.Map
	statsMap  *ebpf.Map
	tcLink    link.Link
	logger    *zap.Logger
}

// Load loads the pre-compiled eBPF object and attaches the TC classifier
// to the given network interface.
func Load(iface string, logger *zap.Logger) (*Program, error) {
	// Load the pre-compiled eBPF object
	spec, err := ebpf.LoadCollectionSpec("ebpf/tc_redirect.o")
	if err != nil {
		return nil, fmt.Errorf("failed to load eBPF spec: %w", err)
	}

	coll, err := ebpf.NewCollection(spec)
	if err != nil {
		return nil, fmt.Errorf("failed to create eBPF collection: %w", err)
	}

	prog := coll.Programs["fyvault_redirect"]
	if prog == nil {
		return nil, fmt.Errorf("eBPF program 'fyvault_redirect' not found")
	}

	targetMap := coll.Maps["fyvault_targets"]
	statsMap := coll.Maps["fyvault_stats"]

	// Attach TC classifier to the network interface
	ifaceObj, err := net.InterfaceByName(iface)
	if err != nil {
		return nil, fmt.Errorf("interface %s not found: %w", iface, err)
	}

	tcLink, err := link.AttachTCX(link.TCXOptions{
		Interface: ifaceObj.Index,
		Program:   prog,
		Attach:    ebpf.AttachTCXEgress,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to attach TC program: %w", err)
	}

	logger.Info("eBPF TC program attached", zap.String("interface", iface))

	return &Program{
		iface:     iface,
		targetMap: targetMap,
		statsMap:  statsMap,
		tcLink:    tcLink,
		logger:    logger,
	}, nil
}

func ipToUint32(ip net.IP) uint32 {
	ip4 := ip.To4()
	if ip4 == nil {
		return 0
	}
	return uint32(ip4[0])<<24 | uint32(ip4[1])<<16 | uint32(ip4[2])<<8 | uint32(ip4[3])
}

func htons(v uint16) uint16 {
	return (v >> 8) | (v << 8)
}

func htonl(v uint32) uint32 {
	return (v>>24)&0xff | (v>>8)&0xff00 | (v<<8)&0xff0000 | (v<<24)&0xff000000
}

// AddTarget inserts a destination IP:port -> proxy port mapping into the eBPF map.
func (p *Program) AddTarget(ip net.IP, dstPort, proxyPort uint16) error {
	key := TargetKey{
		DstIP:   htonl(ipToUint32(ip)),
		DstPort: htons(dstPort),
	}
	val := TargetValue{
		ProxyPort: htons(proxyPort),
	}
	return p.targetMap.Put(key, val)
}

// RemoveTarget deletes a destination IP:port mapping from the eBPF map.
func (p *Program) RemoveTarget(ip net.IP, dstPort uint16) error {
	key := TargetKey{
		DstIP:   htonl(ipToUint32(ip)),
		DstPort: htons(dstPort),
	}
	return p.targetMap.Delete(key)
}

// Close detaches the eBPF program from the network interface and releases resources.
func (p *Program) Close() error {
	if p.tcLink != nil {
		p.tcLink.Close()
		p.logger.Info("eBPF TC program detached")
	}
	return nil
}
