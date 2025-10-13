package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
)

func main() {
	var objs xdp_block_countObjects
	if err := loadXdp_block_countObjects(&objs, nil); err != nil {
		fmt.Fprint(os.Stderr, "Loading eBPF objects:", err)
		os.Exit(1)
	}
	defer objs.Close()

	prog := objs.xdp_block_countPrograms.XdpBlockCountProg
	ipCounters := objs.xdp_block_countMaps.IpCounters
	// blocklist := objs.xdp_block_countMaps.Blocklist

	ifname := "enp0s1"
	iface, err := net.InterfaceByName(ifname)
	if err != nil {
		log.Fatalf("Getting interface %s: %s", ifname, err)
	}

	// Attach XDP to interface
	l, err := link.AttachXDP(link.XDPOptions{
		Program:   prog,
		Interface: iface.Index,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "attach XDP: %v\n", err)
		os.Exit(1)
	}
	defer l.Close()

	fmt.Printf("Attached XDP program to %s (ifindex %d)\n", ifname, iface.Index)

	// Periodically iterate over ip_counters and print top N
	ticker := time.NewTicker(time.Duration(2) * time.Second)
	defer ticker.Stop()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	fmt.Println("Press Ctrl-C to detach and exit")

	for {
		select {
		case <-ticker.C:
			printTop(ipCounters, 10)
		case <-sig:
			fmt.Println("exiting, detaching XDP...")
			return
		}
	}
}

func printTop(m *ebpf.Map, topN int) {
	// iterate map
	iter := m.Iterate()
	var key uint32
	var val uint64
	type kv struct {
		ip  uint32
		cnt uint64
	}
	var arr []kv
	for iter.Next(&key, &val) {
		arr = append(arr, kv{ip: key, cnt: val})
	}
	if err := iter.Err(); err != nil {
		fmt.Printf("map iterate error: %v\n", err)
		return
	}
	sort.Slice(arr, func(i, j int) bool { return arr[i].cnt > arr[j].cnt })
	fmt.Printf("Top %d sources (time=%s):\n", topN, time.Now().Format(time.RFC3339))
	for i := 0; i < len(arr) && i < topN; i++ {
		fmt.Printf("%2d) %s: %d\n", i+1, uint32ToIP(arr[i].ip).String(), arr[i].cnt)
	}
	if len(arr) == 0 {
		fmt.Println("  (no counters yet)")
	}
}

func uint32ToIP(u uint32) net.IP {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, u)
	return net.IP(b)
}
