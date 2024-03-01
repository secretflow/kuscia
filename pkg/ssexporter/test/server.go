// Copyright 2023 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
