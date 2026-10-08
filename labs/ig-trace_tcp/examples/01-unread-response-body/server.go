// The "orders API": GET /orders/{id} returns the order as a ~1KB JSON
// document.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

type Order struct {
	ID    string   `json:"id"`
	Items []string `json:"items"`
	Note  string   `json:"note"`
}

func main() {
	http.HandleFunc("/orders/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/orders/")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Order{
			ID:    id,
			Items: []string{"widget", "gadget", "gizmo"},
			Note:  strings.Repeat("x", 1024),
		})
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}
