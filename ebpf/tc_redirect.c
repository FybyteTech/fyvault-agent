// SPDX-License-Identifier: GPL-2.0
// FyVault TC egress classifier - redirects outbound connections to local proxies

#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <linux/tcp.h>
#include <linux/in.h>
#include <linux/pkt_cls.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

// Map: destination IP:port -> local proxy port
struct target_key {
    __be32 dst_ip;
    __be16 dst_port;
    __u16  pad;
};

struct target_value {
    __be16 proxy_port;
    __u16  pad;
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 256);
    __type(key, struct target_key);
    __type(value, struct target_value);
} fyvault_targets SEC(".maps");

// Stats map for monitoring
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, 2); // 0=packets_redirected, 1=packets_passed
    __type(key, __u32);
    __type(value, __u64);
} fyvault_stats SEC(".maps");

SEC("tc")
int fyvault_redirect(struct __sk_buff *skb) {
    void *data = (void *)(long)skb->data;
    void *data_end = (void *)(long)skb->data_end;

    // Parse Ethernet header
    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end)
        return TC_ACT_OK;

    // Only handle IPv4 for now
    if (eth->h_proto != bpf_htons(ETH_P_IP))
        return TC_ACT_OK;

    // Parse IP header
    struct iphdr *ip = (void *)(eth + 1);
    if ((void *)(ip + 1) > data_end)
        return TC_ACT_OK;

    // Only handle TCP
    if (ip->protocol != IPPROTO_TCP)
        return TC_ACT_OK;

    // Parse TCP header
    struct tcphdr *tcp = (void *)ip + (ip->ihl * 4);
    if ((void *)(tcp + 1) > data_end)
        return TC_ACT_OK;

    // Look up destination in our redirect map
    struct target_key key = {
        .dst_ip = ip->daddr,
        .dst_port = tcp->dest,
        .pad = 0,
    };

    struct target_value *val = bpf_map_lookup_elem(&fyvault_targets, &key);
    if (!val) {
        // No redirect needed - pass through
        __u32 idx = 1;
        __u64 *count = bpf_map_lookup_elem(&fyvault_stats, &idx);
        if (count) __sync_fetch_and_add(count, 1);
        return TC_ACT_OK;
    }

    // Redirect: rewrite destination to 127.0.0.1:proxy_port
    // Save old values for checksum recalculation
    __be32 old_daddr = ip->daddr;
    __be16 old_dport = tcp->dest;

    // New destination: loopback
    __be32 new_daddr = bpf_htonl(0x7f000001); // 127.0.0.1
    __be16 new_dport = val->proxy_port;

    // Update IP destination
    ip->daddr = new_daddr;

    // Recalculate IP checksum (incremental)
    bpf_l3_csum_replace(skb,
        sizeof(struct ethhdr) + __builtin_offsetof(struct iphdr, check),
        old_daddr, new_daddr, sizeof(__be32));

    // Update TCP destination port
    tcp->dest = new_dport;

    // Recalculate TCP checksum (incremental)
    bpf_l4_csum_replace(skb,
        (void *)tcp - data + __builtin_offsetof(struct tcphdr, check),
        old_daddr, new_daddr, BPF_F_PSEUDO_HDR | sizeof(__be32));
    bpf_l4_csum_replace(skb,
        (void *)tcp - data + __builtin_offsetof(struct tcphdr, check),
        old_dport, new_dport, sizeof(__be16));

    // Update stats
    __u32 idx = 0;
    __u64 *count = bpf_map_lookup_elem(&fyvault_stats, &idx);
    if (count) __sync_fetch_and_add(count, 1);

    return TC_ACT_OK;
}

char _license[] SEC("license") = "GPL";
