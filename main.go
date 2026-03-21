package main

import (
	"log"
	"net/http"
)

func main() {
	// Point to the "static" directory
	fs := http.FileServer(http.Dir("./static"))
	
	// Route all traffic to the file server
	http.Handle("/", fs)

	log.Println("Starting placeholder server on :8080...")
	
	// Listen on port 8080
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}