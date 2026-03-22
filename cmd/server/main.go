package main

import (
	"log"
	"net/http"
)

func main() {
	log.Println("TaskHub V2 starting...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
