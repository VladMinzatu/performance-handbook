package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/cilium/ebpf/ringbuf"
	"golang.org/x/sys/unix"
)

const (
	BpfObjectFilename = "profile_bpfel.o"
	PerfFreq          = 49 // (Hz)
)

// Mirrors the C struct layout in profile.c.
type Event struct {
	Pid              uint32
	Cpu              uint32
	TsNs             uint64
	UserStackBytes   int32
	KernelStackBytes int32
	// follow with user_stack and kernel_stack arrays of uint64
	// MAX_STACK_FRAMES == 64 in C
	UserStack   [64]uint64
	KernelStack [64]uint64
}

func main() {
	if os.Geteuid() != 0 {
		log.Fatalf("this program needs to run as root (to open perf events and load eBPF)")
	}

	pid := flag.Int("pid", 0, "The process ID to be profiled (required)")
	flag.Parse()
	if *pid == 0 {
		fmt.Fprintln(os.Stderr, "Error: -pid flag is required and must be > 0")
		flag.Usage()
		os.Exit(1)
	}

	var objs profileObjects
	if err := loadProfileObjects(&objs, nil); err != nil {
		log.Fatal("Loading eBPF objects:", err)
	}
	defer objs.Close()

	prog := objs.profilePrograms.Sample
	eventsMap := objs.profileMaps.Events

	reader, err := ringbuf.NewReader(eventsMap)
	if err != nil {
		log.Fatalf("creating ringbuf reader: %v", err)
	}
	defer reader.Close()

	// Attach perf events on each CPU: open perf_events with sample_freq and attach program via ioctl
	numCPU := runtime.NumCPU()
	perfFds := make([]int, 0, numCPU)
	for cpu := 0; cpu < 1; cpu++ {
		fd, err := openPerfEventFreq(PerfFreq, *pid, cpu)
		if err != nil {
			// cleanup fds we've opened
			for _, f := range perfFds {
				unix.Close(f)
			}
			log.Fatalf("perf_event_open cpu %d: %v", cpu, err)
		}

		// attach BPF program to perf fd
		if err := unix.IoctlSetInt(fd, unix.PERF_EVENT_IOC_SET_BPF, prog.FD()); err != nil {
			unix.Close(fd)
			for _, f := range perfFds {
				unix.Close(f)
			}
			log.Fatalf("ioctl SET_BPF cpu %d: %v", cpu, err)
		}

		// enable the event
		if err := unix.IoctlSetInt(fd, unix.PERF_EVENT_IOC_ENABLE, 0); err != nil {
			unix.Close(fd)
			for _, f := range perfFds {
				unix.Close(f)
			}
			log.Fatalf("ioctl ENABLE cpu %d: %v", cpu, err)
		}

		perfFds = append(perfFds, fd)
	}

	log.Printf("attached sampling program at ~%d Hz to %d CPUs; reading events...", PerfFreq, numCPU)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Reader goroutine
	go func() {
		for {
			rec, err := reader.Read()
			if err != nil {
				if err == ringbuf.ErrClosed {
					return
				}
				log.Printf("ringbuf read error: %v", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}

			var e Event
			if err := binary.Read(bytes.NewReader(rec.RawSample), binary.LittleEndian, &e); err != nil {
				log.Printf("binary.Read event: %v", err)
				continue
			}

			printEvent(&e)
		}
	}()

	<-stop
	log.Println("stopping, detaching perf events...")

	for _, fd := range perfFds {
		_ = unix.IoctlSetInt(fd, unix.PERF_EVENT_IOC_DISABLE, 0)
		unix.Close(fd)
	}

	_ = reader.Close()
	time.Sleep(100 * time.Millisecond)
}

func printEvent(e *Event) {
	fmt.Printf("--- sample pid=%d cpu=%d ts=%dns user_bytes=%d kern_bytes=%d\n",
		e.Pid, e.Cpu, e.TsNs, e.UserStackBytes, e.KernelStackBytes)

	if e.UserStackBytes > 0 {
		fmt.Printf(" user stack (addrs):\n")
		for i, addr := range e.UserStack {
			if addr == 0 {
				break
			}
			fmt.Printf("  [%02d] 0x%016x\n", i, addr)
		}
	}
	if e.KernelStackBytes > 0 {
		fmt.Printf(" kernel stack (addrs):\n")
		for i, addr := range e.KernelStack {
			if addr == 0 {
				break
			}
			fmt.Printf("  [%02d] 0x%016x\n", i, addr)
		}
	}
}

func openPerfEventFreq(freq int, pid int, cpu int) (int, error) {
	attr := unix.PerfEventAttr{
		Type:        unix.PERF_TYPE_SOFTWARE,
		Config:      unix.PERF_COUNT_SW_CPU_CLOCK,
		Sample:      uint64(freq),
		Sample_type: unix.PERF_SAMPLE_IP,
	}

	fd, err := unix.PerfEventOpen(&attr, pid, cpu, -1, 0)
	if err != nil {
		return -1, fmt.Errorf("perf_event_open: %w", err)
	}
	return fd, nil
}
