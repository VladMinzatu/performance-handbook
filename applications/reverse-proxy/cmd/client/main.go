package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
)

func main() {
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	requests := []string{"Hello 1\n", "Hello 2\n", "Hello 3\n"}
	for _, req := range requests {
		_, err := conn.Write([]byte(req))
		if err != nil {
			log.Printf("Failed to send request: %v", err)
			continue
		}

		resp, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			log.Printf("Failed to read response: %v", err)
			continue
		}
		fmt.Printf("Received: %s", resp)
	}
}
