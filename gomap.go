package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	green = "\033[32m"
	reset = "\033[0m"
)

func init() {
	flag.Usage = func() {
		banner := `
  ________          _____                 
 /  _____/  ____   /     \ _____  ______  
/   \  ___ /  _ \ /  \ /  \\__  \ \____ \ 
\    \_\  (  <_> )    Y    \/ __ \|  |_> >
 \______  /\____/\____|__  (____  /   __/ 
        \/               \/     \/|__|   
        `
		fmt.Println(banner)
		fmt.Println("Usage: gomap [options]")
		flag.PrintDefaults()
	}
}

func grabBanner(host string, port int) {
	address := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", address, 1*time.Second)
	if err != nil {
		return
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buffer := make([]byte, 4096)
	n, err := conn.Read(buffer)
	if err == nil && n > 0 {
		fmt.Printf("[Banner %d] %s\n", port, strings.TrimSpace(string(buffer[:n])))
	}
}

func scanTCPPorts(host string, ports []int, showBanners bool, threads int) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, threads) // limit concurrency

	for _, port := range ports {
		wg.Add(1)
		sem <- struct{}{} // occupy a slot

		go func(p int) {
			defer wg.Done()
			defer func() { <-sem }() // free slot

			address := fmt.Sprintf("%s:%d", host, p)
			conn, err := net.DialTimeout("tcp", address, 500*time.Millisecond)
			if err == nil {
				fmt.Printf(green+"TCP %d is open\n"+reset, p)
				conn.Close()

				if showBanners {
					grabBanner(host, p)
				}
			}
		}(port)
	}

	wg.Wait()
}

func scanUDPPorts(host string, ports []int, threads int) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, threads)

	for _, port := range ports {
		wg.Add(1)
		sem <- struct{}{}

		go func(p int) {
			defer wg.Done()
			defer func() { <-sem }()

			addr := fmt.Sprintf("%s:%d", host, p)
			conn, err := net.DialTimeout("udp", addr, 500*time.Millisecond)
			if err != nil {
				return
			}

			_, err = conn.Write([]byte("ping"))
			if err != nil {
				conn.Close()
				return
			}

			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			buf := make([]byte, 1024)
			_, err = conn.Read(buf)

			if err == nil {
				fmt.Printf(green+"UDP %d is open or responding\n"+reset, p)
			}

			conn.Close()
		}(port)
	}

	wg.Wait()
}

func parsePorts(rangeStr string) ([]int, error) {
	parts := strings.Split(rangeStr, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid port range: %s", rangeStr)
	}
	start, err1 := strconv.Atoi(parts[0])
	end, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || start < 1 || end > 65535 || start > end {
		return nil, fmt.Errorf("invalid port numbers: %s", rangeStr)
	}

	var ports []int
	for i := start; i <= end; i++ {
		ports = append(ports, i)
	}
	return ports, nil
}

func main() {
	var host, portRange string
	var udp, banners bool
	var threads int

	flag.StringVar(&host, "host", "localhost", "Host to scan")
	flag.StringVar(&portRange, "ports", "1-1024", "Port range to scan (e.g., 20-80)")
	flag.BoolVar(&udp, "udp", false, "Use UDP instead of TCP")
	flag.BoolVar(&banners, "banners", false, "Try to grab service banners on open TCP ports")
	flag.IntVar(&threads, "threads", 100, "Number of concurrent scans")
	flag.Parse()

	if len(os.Args) == 1 {
		flag.Usage()
		os.Exit(1)
	}

	ports, err := parsePorts(portRange)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	if udp {
		fmt.Printf("Starting UDP scan on %s (%d ports) with %d threads...\n", host, len(ports), threads)
		scanUDPPorts(host, ports, threads)
	} else {
		fmt.Printf("Starting TCP scan on %s (%d ports) with %d threads...\n", host, len(ports), threads)
		scanTCPPorts(host, ports, banners, threads)
	}
}
