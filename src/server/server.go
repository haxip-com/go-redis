package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
)

const SERVER_PORT = "6379"

type Command int

const (
	ERROR Command = iota
	PING
	GET
	SET
	DEL
)

func parseCommand(buf string) (Command, []string, error) {
	buf = strings.TrimSpace(buf)
	if buf == "" {
		return ERROR, nil, fmt.Errorf("empty command")
	}
	parts := strings.Split(buf, " ")
	command := strings.ToUpper(parts[0])
	args := parts[1:]

	// check if command is emtpy
	switch command {
	case "PING":
		return PING, nil, nil
	case "GET":
		return GET, args, nil
	case "SET":
		return SET, args, nil
	case "DEL":
		return DEL, args, nil
	default:
		return ERROR, nil, fmt.Errorf("unknown command: %s", command)
	}
}

func connHandler(conn net.Conn, mem *Store) {
	defer conn.Close()
	reciever := bufio.NewReader(conn)

	for {
		mess, err := reciever.ReadString('\n')
		if err != nil {
			return
		}

		cmd, args, err := parseCommand(mess)
		if err != nil || cmd == ERROR {
			log.Printf("Parse error: %v", err)
			fmt.Fprintf(conn, "Parse error\n")
			continue
		}

		switch cmd {
		case PING:
			fmt.Fprintf(conn, "Pong\n")
		case GET:
			key := args[0]
			val := mem.Get(key)
			fmt.Fprintf(conn, "%v\n", val)
		case SET:
			key := args[0]
			val := args[1]
			mem.Set(key, []byte(val))
			fmt.Fprintf(conn, "DONE\n")
		case DEL:
			key := args[0]
			mem.Del(key)
			fmt.Fprintf(conn, "DONE\n")
		}
	}
}

func main() {
	log.Println("Starting server.")

	mem := newStore()

	listener, err := net.Listen("tcp", ":"+SERVER_PORT)
	if err != nil {
		log.Fatal("Failed to bind to port "+SERVER_PORT+": ", err)
	}
	log.Println("Server started on port " + SERVER_PORT)
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Error accepting connection:", err)
			continue
		}
		go connHandler(conn, mem)
	}
}
