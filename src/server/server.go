package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"

	"github.com/haxip-com/go-redis/src/parser"
)

const (
	SERVER_PORT   = "6379"
	READ_TIMEOUT  = 5 * time.Minute
	WRITE_TIMEOUT = 10 * time.Second
)

type CommandHandler func(store *Store, args []parser.Value) parser.Value

type CommandSpec struct {
	handler CommandHandler
	arity   int // positive = exact, negative = minimum (abs(arity)-1)
}

var commands = map[string]CommandSpec{
	"PING": {handlePing, 1},
	"ECHO": {handleEcho, 2},
	"GET":  {handleGet, 2},
	"SET":  {handleSet, 3},
	"DEL":  {handleDel, -2},
	"INCR": {handleIncr, 2},
	"DECR": {handleDecr, 2},
}

func handlePing(store *Store, args []parser.Value) parser.Value {
	return parser.SimpleString("PONG")
}

func handleEcho(store *Store, args []parser.Value) parser.Value {
	if bs, ok := args[1].(parser.BulkString); ok {
		return bs
	}
	return parser.Error("ERR wrong argument type")
}

func handleGet(store *Store, args []parser.Value) parser.Value {
	bs, ok := args[1].(parser.BulkString)
	if !ok {
		return parser.Error("ERR wrong argument type")
	}
	val, exists := store.Get(string(bs))
	if !exists {
		return parser.BulkString(nil)
	}
	return parser.BulkString(val)
}

func handleSet(store *Store, args []parser.Value) parser.Value {
	key, ok1 := args[1].(parser.BulkString)
	val, ok2 := args[2].(parser.BulkString)
	if !ok1 || !ok2 {
		return parser.Error("ERR wrong argument type")
	}
	store.Set(string(key), []byte(val))
	return parser.SimpleString("OK")
}

func handleDel(store *Store, args []parser.Value) parser.Value {
	keys := make([]string, 0, len(args)-1)
	for i := 1; i < len(args); i++ {
		if bs, ok := args[i].(parser.BulkString); ok {
			keys = append(keys, string(bs))
		} else {
			return parser.Error("ERR wrong argument type")
		}
	}
	count := store.Del(keys...)
	return parser.Integer(count)
}

func handleIncr(store *Store, args []parser.Value) parser.Value {
	key, ok := args[1].(parser.BulkString)
	if !ok {
		return parser.Error("ERR wrong argument type")
	}

	newVal, err := store.Incr(string(key))
	if err != nil {
		return parser.Error(err.Error())
	}
	return parser.Integer(newVal)
}

func handleDecr(store *Store, args []parser.Value) parser.Value {
	key, ok := args[1].(parser.BulkString)
	if !ok {
		return parser.Error("ERR wrong argument type")
	}

	newVal, err := store.Decr(string(key))
	if err != nil {
		return parser.Error(err.Error())
	}
	return parser.Integer(newVal)
}

func connHandler(conn net.Conn, store *Store) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	for {
		conn.SetReadDeadline(time.Now().Add(READ_TIMEOUT))
		value, err := parser.Deserialize(reader)
		if err != nil {
			if err == io.EOF {
				return
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				return
			}
			return
		}

		arr, ok := value.(parser.Array)
		if !ok || len(arr) == 0 {
			reply, _ := parser.Serialize(parser.Error("ERR protocol error"))
			conn.SetWriteDeadline(time.Now().Add(WRITE_TIMEOUT))
			conn.Write(reply)
			continue
		}

		cmdName, ok := arr[0].(parser.BulkString)
		if !ok {
			reply, _ := parser.Serialize(parser.Error("ERR protocol error"))
			conn.SetWriteDeadline(time.Now().Add(WRITE_TIMEOUT))
			conn.Write(reply)
			continue
		}

		cmd := strings.ToUpper(string(cmdName))
		spec, exists := commands[cmd]
		if !exists {
			reply, _ := parser.Serialize(parser.Error(fmt.Sprintf("ERR unknown command '%s'", cmd)))
			conn.SetWriteDeadline(time.Now().Add(WRITE_TIMEOUT))
			conn.Write(reply)
			continue
		}

		if spec.arity > 0 && len(arr) != spec.arity {
			reply, _ := parser.Serialize(parser.Error(fmt.Sprintf("ERR wrong number of arguments for '%s' command", cmd)))
			conn.SetWriteDeadline(time.Now().Add(WRITE_TIMEOUT))
			conn.Write(reply)
			continue
		} else if spec.arity < 0 && len(arr) < -spec.arity {
			reply, _ := parser.Serialize(parser.Error(fmt.Sprintf("ERR wrong number of arguments for '%s' command", cmd)))
			conn.SetWriteDeadline(time.Now().Add(WRITE_TIMEOUT))
			conn.Write(reply)
			continue
		}

		result := spec.handler(store, arr)
		reply, _ := parser.Serialize(result)
		conn.SetWriteDeadline(time.Now().Add(WRITE_TIMEOUT))
		conn.Write(reply)
	}
}

func main() {
	log.Println("Starting server.")

	store := newStore()

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
		go connHandler(conn, store)
	}
}
