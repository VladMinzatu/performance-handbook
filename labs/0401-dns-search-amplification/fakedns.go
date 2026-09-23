// Minimal authoritative DNS UDP server for lab 0401. Answers exactly one
// configured FQDN (TARGET_FQDN) with a fixed A record; everything else -
// wrong name, wrong record type - gets NXDOMAIN. Logs every query it
// receives, independent of whatever `ig trace_dns` sees, so the two can
// be cross-checked against each other.
//
// Deliberately hand-rolled rather than using a DNS library: parsing just
// the question section (name + qtype) and echoing back a minimal
// response is enough here, and keeps the whole server legible in one
// file.
package main

import (
	"fmt"
	"net"
	"os"
)

func main() {
	target := os.Getenv("TARGET_FQDN")
	addr, err := net.ResolveUDPAddr("udp", ":53")
	if err != nil {
		panic(err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	fmt.Println("fakedns listening, target =", target)

	buf := make([]byte, 512)
	for {
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}
		msg := buf[:n]
		qname, qtype, ok := parseQuestion(msg)
		if !ok {
			continue
		}
		fmt.Printf("query: %s (type=%d)\n", qname, qtype)

		var resp []byte
		if qname == target && qtype == 1 { // 1 = A record
			resp = buildAResponse(msg, "10.0.0.99")
		} else {
			resp = buildNXDOMAIN(msg)
		}
		conn.WriteToUDP(resp, remote)
	}
}

// parseQuestion extracts the queried name and record type from a DNS
// request's question section (the only part this server needs to read).
func parseQuestion(msg []byte) (name string, qtype uint16, ok bool) {
	if len(msg) < 12 {
		return "", 0, false
	}
	pos := 12
	var labels []string
	for pos < len(msg) {
		l := int(msg[pos])
		if l == 0 {
			pos++
			break
		}
		pos++
		if pos+l > len(msg) {
			return "", 0, false
		}
		labels = append(labels, string(msg[pos:pos+l]))
		pos += l
	}
	if pos+4 > len(msg) {
		return "", 0, false
	}
	qtype = uint16(msg[pos])<<8 | uint16(msg[pos+1])
	for i, l := range labels {
		if i > 0 {
			name += "."
		}
		name += l
	}
	return name, qtype, true
}

// buildAResponse turns the original query into a response in place: it
// reuses the query's header/question section (same transaction ID, same
// question) and appends one A-record answer.
func buildAResponse(query []byte, ip string) []byte {
	resp := make([]byte, len(query))
	copy(resp, query)
	resp[2] = 0x81 // QR=1 (response), RD=1
	resp[3] = 0x80 // RA=1, RCODE=0 (NOERROR)
	resp[6], resp[7] = 0, 1 // ANCOUNT=1
	ipParts := net.ParseIP(ip).To4()
	answer := []byte{0xc0, 0x0c, 0, 1, 0, 1, 0, 0, 0, 60, 0, 4} // name ptr, TYPE=A, CLASS=IN, TTL=60, RDLENGTH=4
	answer = append(answer, ipParts...)
	return append(resp, answer...)
}

// buildNXDOMAIN echoes the query back as a response with RCODE=3.
func buildNXDOMAIN(query []byte) []byte {
	resp := make([]byte, len(query))
	copy(resp, query)
	resp[2] = 0x81
	resp[3] = 0x83 // RCODE=3 (NXDOMAIN)
	return resp
}
