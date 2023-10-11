package main

import (
	"fmt"
	"net"
	"time"
)

func main() {
	numConnections := 1000000

	serverAddr := "172.18.0.4:9523"

	for i := 0; i < numConnections; i++ {
		go func(connectionNumber int) {
			conn, err := net.Dial("tcp", serverAddr)
			if err != nil {
				fmt.Printf("连接 %d 失败: %v\n", connectionNumber, err)
				return
			}
			defer conn.Close()
			if connectionNumber%5000 == 0 {
				fmt.Printf("连接 %d 成功\n", connectionNumber)
			}
		}(i + 1)
		if i%10000 == 0 {
			time.Sleep(2 * time.Second)
		}
	}

	time.Sleep(10 * time.Second)
}
