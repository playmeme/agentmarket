package main

import (
    "encoding/json"
    "log"
    "net/http"
)

func main() {
    mux := http.NewServeMux()

    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]string{
            "status":  "healthy",
            "service": "static-web-server",
        })
    })

    fs := http.FileServer(http.Dir("./static"))
    mux.Handle("/", fs)

    log.Println("Starting placeholder server on :8080...")
    err := http.ListenAndServe(":8080", mux)
    if err != nil {
        log.Fatal(err)
    }
}
