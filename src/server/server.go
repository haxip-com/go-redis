package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
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
	"CONFIG": {handleConfig,-2},
	"EXPIRE": {handleExpire, -3},
	"EXPIREAT":{handleExpire, -3},

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
	fmt.Println("called")
	bs, ok := args[1].(parser.BulkString)
	if !ok {
		fmt.Println("maybe here")
		return parser.Error("ERR wrong argument type")
	}
	val, exists := store.Get(string(bs))
	if !exists {
		fmt.Println("does not exist")
		return parser.BulkString(nil)
	}
	fmt.Println("passing here")
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

func handleConfig(store *Store, args []parser.Value) parser.Value {
	if s, ok := args[0].(parser.BulkString); ok && string(s) == "CONFIG" {
        // Accept any number of arguments, or ignore them for benchmarking
		arr := []parser.Value{
			parser.BulkString("maxmemory"),
			parser.BulkString("0"),
		}
		return parser.Array(arr)
    }
	return parser.Error("ERR invalid format")
}

func getSetterAndDuration(command string, t int64, store *Store) (expirationSetter, time.Duration) {
	switch command {
	case "EXPIRE":
		duration := time.Duration(t) * time.Second
		setter := withTTL(&store.volatileKeyMap, duration)
		return setter, duration
	case "EXPIREAT":
		converted_t := time.Unix(t, 0)
		setter, duration := withUnixExpiry(&store.volatileKeyMap, converted_t)
		return setter, duration
	default:
		return nil, time.Duration(time.Now().Day())
	}
}

func handleExpire(store *Store, args []parser.Value) parser.Value {
	command := args[0].(parser.BulkString)
	timeString, ok := args[2].(parser.BulkString)
		if !ok {
			return parser.Error("ERR wrong argument type")
		}
	t, err := strconv.ParseInt(string(timeString), 10, 64)
		if  err != nil {
			return parser.Error("ERR wrong argument type")
		}
	key, ok := args[1].(parser.BulkString)
	if !ok {
		return parser.Error("ERR wrong argument type")
	}
	switch string(command) {
	case "EXPIRE":
		if t < 0 {
			store.volatileKeyMap.Delete(string(key))
			return parser.Integer(0)
		}
	case "EXPIREAT":
		if time.Unix(t, 0).Before(time.Now()) {
			store.volatileKeyMap.Delete(string(key))
			return parser.Integer(1)
		}
	default:
		return parser.Error("ERR wrong argument type")
	}
	setter, duration:= getSetterAndDuration(string(command), t, store)
	if len(args) < 3 {
		return parser.Error("ERR wrong number of arguments for EXPIRE command")
	}
	if len(args) > 3 {
		optionBulkString, ok := args[3].(parser.BulkString)
		if !ok {
			return parser.Error("ERR wrong argument type")
		}
		option := string(optionBulkString)
		res := handleExpireOption(store, option, string(key), setter, duration)
		return res

	} else {
		store.volatileKeyMap.apply(string(key), setter)
		return parser.Integer(1)
	}
}

func handleExpireOption(store *Store, option string, key string, setter expirationSetter, duration time.Duration) parser.Value {
	switch  option {
		case "NX":
			if  store.isVolatile(string(key)){
				return parser.Integer(0)
			} else {
				store.volatileKeyMap.apply(string(key), setter)
				return parser.Integer(1)
			}
		case "XX":
			if  store.isVolatile(string(key)){
				store.volatileKeyMap.apply(string(key), setter)
				return parser.Integer(1)
			} else {
				return parser.Integer(0)
			}
		case "GT":
			if store.isVolatile(string(key)){
				originalDuration, ok :=store.volatileKeyMap.GetDuration(string(key))
				if ok==nil && duration > originalDuration {
					store.volatileKeyMap.apply(string(key), setter)
					return parser.Integer(1)
				} else {
					return parser.Integer(0)
				}

			} else {
				return parser.Integer(0)
			}
		case "LT":
			if store.isVolatile(string(key)){
				originalDuration, ok :=store.volatileKeyMap.GetDuration(string(key))
				if ok==nil && duration < originalDuration {
					store.volatileKeyMap.apply(string(key), setter)
					return parser.Integer(1)
				} else {
					return parser.Integer(0)
				}
			} else {
				return parser.Integer(0)
			}
		default:
			return parser.Error("ERR Unsupported option " + option)
		}
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

		var cmdName string
		switch v := arr[0].(type) {
		case parser.BulkString:
			cmdName = string(v)
		case parser.SimpleString:
			cmdName = string(v)
		default:
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
		fmt.Println("writing to conn")
		if _, err := conn.Write(reply); err != nil {
    		log.Println("write error:", err)
    		return
		}
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
