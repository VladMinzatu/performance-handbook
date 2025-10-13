package main

import (
	"fmt"
	"log"
	"net"
	"os"

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
}
