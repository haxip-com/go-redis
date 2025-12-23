package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/haxip-com/go-redis/src/parser"
)

func printValue(v parser.Value) {
	switch t := v.(type) {
	case parser.SimpleString:
		fmt.Println(string(t))
	case parser.Error:
		fmt.Println("(error)", string(t))
	case parser.Integer:
		fmt.Printf("(integer) %d\n", t)
	case parser.BulkString:
		if t == nil {
			fmt.Println("(nil)")
		} else {
			fmt.Println(string(t))
		}
	case parser.Array:
		for i, elem := range t {
			fmt.Printf("%d) ", i+1)
			printValue(elem)
		}
	default:
		fmt.Printf("%v\n", v)
	}
}

func main() {
	conn, err := net.Dial("tcp", "localhost:6379")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	scanner := bufio.NewScanner(os.Stdin)
	reader := bufio.NewReader(conn)

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		serialized, err := parser.SerializeFromString(scanner.Text())
		if err != nil {
			log.Println("Error:", err)
			continue
		}

		_, err = conn.Write(serialized)
		if err != nil {
			log.Fatal(err)
		}

		resp, err := parser.Deserialize(reader)
		if err != nil {
			log.Fatal(err)
		}
		printValue(resp)
	}
}
