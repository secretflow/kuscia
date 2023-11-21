package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

// w表示response对象，返回给客户端的内容都在对象里处理
// r表示客户端请求对象，包含了请求头，请求参数等等
func index(w http.ResponseWriter, r *http.Request) {
	// 往w里写入内容，就会在浏览器里输出
	_, err := ioutil.ReadAll(r.Body)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		fmt.Fprintf(w, "Hello World")
	} else {
		//http.Error(w, "{\"detail\":Could not read the given request body}", http.StatusBadRequest)
		fmt.Fprintf(w, "{\"haha\": {\"age\":121},\"hehe\":\"dsds\"}")
	}

}

func main() {
	// 设置路由，如果访问/，则调用index方法
	http.HandleFunc("/", index)

	// 启动web服务，监听9090端口
	err := http.ListenAndServe("127.0.0.1:9812", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
		fmt.Println("listen fail")
	} else {
		fmt.Println("listen succ")
	}
}
