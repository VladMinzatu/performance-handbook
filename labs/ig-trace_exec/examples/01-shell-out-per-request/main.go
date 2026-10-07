// A checksum service: GET /checksum?id=N returns the SHA-256 of document N
// (a 4KB payload derived from N).
//
// MODE=shell (default) pipes the document through `sh -c sha256sum` - the
// "legacy code shells out to a CLI tool" pattern. MODE=native hashes it
// in-process with crypto/sha256.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

func document(id string) []byte {
	return bytes.Repeat([]byte(fmt.Sprintf("document-%s\n", id)), 4096/(len(id)+10))
}

func shellSum(doc []byte) (string, error) {
	cmd := exec.Command("sh", "-c", "sha256sum")
	cmd.Stdin = bytes.NewReader(doc)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.Fields(string(out))[0], nil
}

func nativeSum(doc []byte) (string, error) {
	sum := sha256.Sum256(doc)
	return hex.EncodeToString(sum[:]), nil
}

func main() {
	mode := os.Getenv("MODE")
	if mode == "" {
		mode = "shell"
	}
	sum := shellSum
	if mode == "native" {
		sum = nativeSum
	}
	log.Printf("mode=%s listening on :8080", mode)

	http.HandleFunc("/checksum", func(w http.ResponseWriter, r *http.Request) {
		s, err := sum(document(r.URL.Query().Get("id")))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintln(w, s)
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}
