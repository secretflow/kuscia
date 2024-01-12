package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

func index(w http.ResponseWriter, r *http.Request) {
	_, err := ioutil.ReadAll(r.Body)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		_, err := fmt.Fprintf(w, "Hello World")
		if err != nil {
			return
		}
	} else {
		_, err2 := fmt.Fprintf(w, "{\"haha\": {\"age\":121},\"hehe\":\"dsds\"}")
		if err2 != nil {
			return
		}
	}

}

func main() {

	http.HandleFunc("/", index)

	err := http.ListenAndServe("127.0.0.1:9812", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	} else {
		fmt.Println("listen success")
	}
}
