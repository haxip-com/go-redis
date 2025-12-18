package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
)

func main() {
	conn, err := net.Dial("tcp", "localhost:6379")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	scanner := bufio.NewScanner(os.Stdin)
	reciever := bufio.NewReader(conn)

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		fmt.Fprintf(conn, "%s\n", scanner.Text())
		resp, err := reciever.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}
		fmt.Print(resp)

	}
}
