//go:build linux

package ebpf

import (
	"fmt"
	"net"
	"os"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"go.uber.org/zap"
)

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

// Load loads the eBPF object and attaches the TC classifier to the given
// network interface.
//
// It searches for the compiled tc_redirect.o in multiple paths:
//  1. ebpf/tc_redirect.o (relative to working dir — dev)
//  2. /usr/lib/fyvault/tc_redirect.o (installed via package manager)
//  3. /opt/fyvault/ebpf/tc_redirect.o (Docker/manual install)
func Load(iface string, logger *zap.Logger) (*Program, error) {
	paths := []string{
		"ebpf/tc_redirect.o",
		"/usr/lib/fyvault/tc_redirect.o",
		"/opt/fyvault/ebpf/tc_redirect.o",
	}

	var spec *ebpf.CollectionSpec
	var loadErr error
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			logger.Info("Loading eBPF object", zap.String("path", path))
			spec, loadErr = ebpf.LoadCollectionSpec(path)
			if loadErr == nil {
				break
			}
			logger.Warn("Failed to load eBPF spec", zap.String("path", path), zap.Error(loadErr))
		}
	}

	if spec == nil {
		if loadErr != nil {
			return nil, fmt.Errorf("failed to load eBPF object: %w", loadErr)
		}
		return nil, fmt.Errorf("eBPF object tc_redirect.o not found in any search path: %v", paths)
	}

	coll, err := ebpf.NewCollection(spec)
	if err != nil {
		return nil, fmt.Errorf("failed to create eBPF collection: %w", err)
	}

	prog := coll.Programs["fyvault_redirect"]
	if prog == nil {
		return nil, fmt.Errorf("eBPF program 'fyvault_redirect' not found in collection")
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
		return nil, fmt.Errorf("failed to attach TC program to %s: %w", iface, err)
	}

	logger.Info("eBPF TC program attached",
		zap.String("interface", iface),
		zap.Bool("has_target_map", targetMap != nil),
		zap.Bool("has_stats_map", statsMap != nil))

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
	if p.targetMap == nil {
		return fmt.Errorf("target map not available")
	}
	key := TargetKey{
		DstIP:   htonl(ipToUint32(ip)),
		DstPort: htons(dstPort),
	}
	val := TargetValue{
		ProxyPort: htons(proxyPort),
	}
	if err := p.targetMap.Put(key, val); err != nil {
		return fmt.Errorf("failed to add target %s:%d -> proxy :%d: %w",
			ip, dstPort, proxyPort, err)
	}
	p.logger.Debug("eBPF target added",
		zap.String("dest", fmt.Sprintf("%s:%d", ip, dstPort)),
		zap.Uint16("proxy_port", proxyPort))
	return nil
}

// RemoveTarget deletes a destination IP:port mapping from the eBPF map.
func (p *Program) RemoveTarget(ip net.IP, dstPort uint16) error {
	if p.targetMap == nil {
		return fmt.Errorf("target map not available")
	}
	key := TargetKey{
		DstIP:   htonl(ipToUint32(ip)),
		DstPort: htons(dstPort),
	}
	return p.targetMap.Delete(key)
}

// Stats returns the packet redirect statistics from the eBPF per-CPU array.
func (p *Program) Stats() (redirected, passed uint64, err error) {
	if p.statsMap == nil {
		return 0, 0, nil
	}
	// percpu array — sum across CPUs
	var idx uint32
	var vals []uint64

	idx = 0
	if err := p.statsMap.Lookup(idx, &vals); err == nil {
		for _, v := range vals {
			redirected += v
		}
	}
	idx = 1
	if err := p.statsMap.Lookup(idx, &vals); err == nil {
		for _, v := range vals {
			passed += v
		}
	}
	return redirected, passed, nil
}

// Close detaches the eBPF program from the network interface and releases resources.
func (p *Program) Close() error {
	if p.tcLink != nil {
		p.tcLink.Close()
		p.logger.Info("eBPF TC program detached", zap.String("interface", p.iface))
	}
	return nil
}
