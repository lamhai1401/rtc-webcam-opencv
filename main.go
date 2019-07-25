package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

func main() {
	router := mux.NewRouter()

	router.HandleFunc("/", broadcastHandler).Methods("GET")
	router.HandleFunc("/offer", offerHandler).Methods("GET")
	router.HandleFunc("/answer", answerHandler).Methods("POST")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	err := http.ListenAndServeTLS(fmt.Sprintf(":%s", port), "server.crt", "server.key", router)
	// err := http.ListenAndServe(fmt.Sprintf(":%s", port), router)

	if err != nil {
		fmt.Print(err)
	}
}

func broadcastHandler(w http.ResponseWriter, r *http.Request) {
	path := fmt.Sprintf("client.html")
	fmt.Println(path)
	http.ServeFile(w, r, path)
}
