package main

import (
	"bufio"
	"bytes"
	"net"
	"testing"
	"time"
	"github.com/haxip-com/go-redis/src/parser"
)

type testServer struct {
	listener net.Listener
	store    *Store
}

func startTestServer(t *testing.T) *testServer {
	store := newStore()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go connHandler(conn, store)
		}
	}()

	return &testServer{listener: listener, store: store}
}

func (ts *testServer) Addr() string {
	return ts.listener.Addr().String()
}

func (ts *testServer) Close() {
	ts.listener.Close()
}

func sendCmd(t *testing.T, conn net.Conn, reader *bufio.Reader, cmd string) parser.Value {
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	serialized, _ := parser.SerializeFromString(cmd)
	conn.Write(serialized)
	resp, err := parser.Deserialize(reader)
	if err != nil {
		t.Fatalf("deserialize error: %v", err)
	}
	return resp
}

func TestPing(t *testing.T) {
	srv := startTestServer(t)
	defer srv.Close()

	conn, _ := net.Dial("tcp", srv.Addr())
	defer conn.Close()
	reader := bufio.NewReader(conn)

	resp := sendCmd(t, conn, reader, "PING")
	if str, ok := resp.(parser.SimpleString); !ok || str != "PONG" {
		t.Errorf("expected PONG, got %v", resp)
	}
}

func TestEcho(t *testing.T) {
	srv := startTestServer(t)
	defer srv.Close()

	conn, _ := net.Dial("tcp", srv.Addr())
	defer conn.Close()
	reader := bufio.NewReader(conn)

	resp := sendCmd(t, conn, reader, "ECHO hello")
	if bs, ok := resp.(parser.BulkString); !ok || string(bs) != "hello" {
		t.Errorf("expected 'hello', got %v", resp)
	}
}

func TestSetGet(t *testing.T) {
	srv := startTestServer(t)
	defer srv.Close()

	conn, _ := net.Dial("tcp", srv.Addr())
	defer conn.Close()
	reader := bufio.NewReader(conn)

	resp := sendCmd(t, conn, reader, "SET mykey myvalue")
	if str, ok := resp.(parser.SimpleString); !ok || str != "OK" {
		t.Errorf("expected OK, got %v", resp)
	}

	resp = sendCmd(t, conn, reader, "GET mykey")
	if bs, ok := resp.(parser.BulkString); !ok || string(bs) != "myvalue" {
		t.Errorf("expected 'myvalue', got %v", resp)
	}
}

func TestGetMissing(t *testing.T) {
	srv := startTestServer(t)
	defer srv.Close()

	conn, _ := net.Dial("tcp", srv.Addr())
	defer conn.Close()
	reader := bufio.NewReader(conn)

	resp := sendCmd(t, conn, reader, "GET nonexistent")
	if resp != nil {
		t.Errorf("expected nil, got %v", resp)
	}
}

func TestDel(t *testing.T) {
	srv := startTestServer(t)
	defer srv.Close()

	conn, _ := net.Dial("tcp", srv.Addr())
	defer conn.Close()
	reader := bufio.NewReader(conn)

	sendCmd(t, conn, reader, "SET key1 val1")
	sendCmd(t, conn, reader, "SET key2 val2")

	resp := sendCmd(t, conn, reader, "DEL key1 key2 key3")
	if num, ok := resp.(parser.Integer); !ok || num != 2 {
		t.Errorf("expected 2, got %v", resp)
	}

	resp = sendCmd(t, conn, reader, "GET key1")
	if resp != nil {
		t.Errorf("expected nil, got %v", resp)
	}
}

func TestUnknownCommand(t *testing.T) {
	srv := startTestServer(t)
	defer srv.Close()

	conn, _ := net.Dial("tcp", srv.Addr())
	defer conn.Close()
	reader := bufio.NewReader(conn)

	resp := sendCmd(t, conn, reader, "UNKNOWN arg")
	if err, ok := resp.(parser.Error); !ok || !bytes.Contains([]byte(err), []byte("unknown command")) {
		t.Errorf("expected unknown command error, got %v", resp)
	}
}

func TestWrongArity(t *testing.T) {
	srv := startTestServer(t)
	defer srv.Close()

	conn, _ := net.Dial("tcp", srv.Addr())
	defer conn.Close()
	reader := bufio.NewReader(conn)

	resp := sendCmd(t, conn, reader, "SET key")
	if err, ok := resp.(parser.Error); !ok || !bytes.Contains([]byte(err), []byte("wrong number of arguments")) {
		t.Errorf("expected arity error, got %v", resp)
	}

	resp = sendCmd(t, conn, reader, "GET")
	if err, ok := resp.(parser.Error); !ok || !bytes.Contains([]byte(err), []byte("wrong number of arguments")) {
		t.Errorf("expected arity error, got %v", resp)
	}
}

func TestPipelining(t *testing.T) {
	srv := startTestServer(t)
	defer srv.Close()

	conn, _ := net.Dial("tcp", srv.Addr())
	defer conn.Close()
	reader := bufio.NewReader(conn)

	cmd1, _ := parser.SerializeFromString("SET key1 val1")
	cmd2, _ := parser.SerializeFromString("SET key2 val2")
	cmd3, _ := parser.SerializeFromString("GET key1")

	conn.Write(cmd1)
	conn.Write(cmd2)
	conn.Write(cmd3)

	resp1, _ := parser.Deserialize(reader)
	if str, ok := resp1.(parser.SimpleString); !ok || str != "OK" {
		t.Errorf("expected OK, got %v", resp1)
	}

	resp2, _ := parser.Deserialize(reader)
	if str, ok := resp2.(parser.SimpleString); !ok || str != "OK" {
		t.Errorf("expected OK, got %v", resp2)
	}

	resp3, _ := parser.Deserialize(reader)
	if bs, ok := resp3.(parser.BulkString); !ok || string(bs) != "val1" {
		t.Errorf("expected 'val1', got %v", resp3)
	}
}

func TestMultipleConnections(t *testing.T) {
	srv := startTestServer(t)
	defer srv.Close()

	conn1, _ := net.Dial("tcp", srv.Addr())
	defer conn1.Close()
	reader1 := bufio.NewReader(conn1)

	conn2, _ := net.Dial("tcp", srv.Addr())
	defer conn2.Close()
	reader2 := bufio.NewReader(conn2)

	sendCmd(t, conn1, reader1, "SET shared value1")
	resp := sendCmd(t, conn2, reader2, "GET shared")

	if bs, ok := resp.(parser.BulkString); !ok || string(bs) != "value1" {
		t.Errorf("expected 'value1', got %v", resp)
	}
}

func TestClientDisconnect(t *testing.T) {
	srv := startTestServer(t)
	defer srv.Close()

	conn, _ := net.Dial("tcp", srv.Addr())
	reader := bufio.NewReader(conn)

	sendCmd(t, conn, reader, "SET key val")
	conn.Close()

	time.Sleep(100 * time.Millisecond)

	conn2, _ := net.Dial("tcp", srv.Addr())
	defer conn2.Close()
	reader2 := bufio.NewReader(conn2)

	resp := sendCmd(t, conn2, reader2, "GET key")
	if bs, ok := resp.(parser.BulkString); !ok || string(bs) != "val" {
		t.Errorf("expected 'val', got %v", resp)
	}
}

func TestIncr(t *testing.T) {
	srv := startTestServer(t)
	defer srv.Close()

	conn, _ := net.Dial("tcp", srv.Addr())
	defer conn.Close()
	reader := bufio.NewReader(conn)

	resp := sendCmd(t, conn, reader, "INCR counter")
	if num, ok := resp.(parser.Integer); !ok || num != 1 {
		t.Errorf("expected 1, got %v", resp)
	}

	resp = sendCmd(t, conn, reader, "INCR counter")
	if num, ok := resp.(parser.Integer); !ok || num != 2 {
		t.Errorf("expected 2, got %v", resp)
	}
}

func TestDecr(t *testing.T) {
	srv := startTestServer(t)
	defer srv.Close()

	conn, _ := net.Dial("tcp", srv.Addr())
	defer conn.Close()
	reader := bufio.NewReader(conn)

	resp := sendCmd(t, conn, reader, "DECR counter")
	if num, ok := resp.(parser.Integer); !ok || num != -1 {
		t.Errorf("expected -1, got %v", resp)
	}

	resp = sendCmd(t, conn, reader, "DECR counter")
	if num, ok := resp.(parser.Integer); !ok || num != -2 {
		t.Errorf("expected -2, got %v", resp)
	}
}

func TestIncrStringValue(t *testing.T) {
	srv := startTestServer(t)
	defer srv.Close()

	conn, _ := net.Dial("tcp", srv.Addr())
	defer conn.Close()
	reader := bufio.NewReader(conn)

	sendCmd(t, conn, reader, "SET num 10")
	resp := sendCmd(t, conn, reader, "INCR num")
	if num, ok := resp.(parser.Integer); !ok || num != 11 {
		t.Errorf("expected 11, got %v", resp)
	}
}

func TestIncrInvalidValue(t *testing.T) {
	srv := startTestServer(t)
	defer srv.Close()

	conn, _ := net.Dial("tcp", srv.Addr())
	defer conn.Close()
	reader := bufio.NewReader(conn)

	sendCmd(t, conn, reader, "SET key notanumber")
	resp := sendCmd(t, conn, reader, "INCR key")
	if err, ok := resp.(parser.Error); !ok || !bytes.Contains([]byte(err), []byte("not an integer")) {
		t.Errorf("expected integer error, got %v", resp)
	}
}

func TestIncrGetReturnsString(t *testing.T) {
	srv := startTestServer(t)
	defer srv.Close()

	conn, _ := net.Dial("tcp", srv.Addr())
	defer conn.Close()
	reader := bufio.NewReader(conn)

	sendCmd(t, conn, reader, "INCR counter")
	sendCmd(t, conn, reader, "INCR counter")

	resp := sendCmd(t, conn, reader, "GET counter")
	if bs, ok := resp.(parser.BulkString); !ok || string(bs) != "2" {
		t.Errorf("expected '2', got %v", resp)
	}
}

func TestExpire(t *testing.T) {
	srv := startTestServer(t)
	defer srv.Close()

	conn, _ := net.Dial("tcp", srv.Addr())
	defer conn.Close()
	reader := bufio.NewReader(conn)

	resp1 := sendCmd(t, conn, reader, "SET mykey myvalue")
	if str, ok := resp1.(parser.SimpleString); !ok || str != "OK" {
		t.Errorf("expected OK, got %v", resp1)
	}
	resp := sendCmd(t, conn, reader, "EXPIRE mykey 2")
	if _, ok := resp.(parser.Integer); !ok  {
		t.Errorf("expected OK, got %v", resp1)
	}
	i64 := int64(resp.(parser.Integer))
	if i64 != 1 {
		t.Errorf("expected 1, got %v", i64)
	}
	_, ok := srv.store.volatileKeyMap.data["mykey"]
    if !ok {
        t.Fatalf("expected key %q to exist in volatileKeyMap", "mykey")
    }
	time.Sleep(1 * time.Second)
	_, stillThere := srv.store.volatileKeyMap.data["mykey"]
    if !stillThere {
        t.Fatalf("expected key %q to stil exists in volatileKeyMap", "mykey")
    }
	time.Sleep(1 * time.Second)
	_, notExpired := srv.store.volatileKeyMap.data["mykey"]
    if !notExpired {
        t.Fatalf("expected key %q to have been expired in volatileKeyMap", "mykey")
    }
	//GET should not let me access it now as it is expired
	res := sendCmd(t, conn, reader, "GET mykey")
	if res != nil {
		t.Errorf("expected nil, got %v", resp)
	}
}

func TestExpireNX(t *testing.T) {
	srv := startTestServer(t)
	defer srv.Close()

	conn, _ := net.Dial("tcp", srv.Addr())
	defer conn.Close()
	reader := bufio.NewReader(conn)

	sendCmd(t, conn, reader, "SET mykey myvalue")

	// Case 1: key has no expiration -> should set
	resp := sendCmd(t, conn, reader, "EXPIRE mykey 10 NX")
	if i, ok := resp.(parser.Integer); !ok || i != 1 {
		t.Fatalf("expected 1, got %v", resp)
	}

	// Case 2: key already has expiration -> should not set
	resp2 := sendCmd(t, conn, reader, "EXPIRE mykey 20 NX")
	if i, ok := resp2.(parser.Integer); !ok || i != 0 {
		t.Fatalf("expected 0, got %v", resp2)
	}
}

func TestExpireXX(t *testing.T) {
	srv := startTestServer(t)
	defer srv.Close()

	conn, _ := net.Dial("tcp", srv.Addr())
	defer conn.Close()
	reader := bufio.NewReader(conn)

	sendCmd(t, conn, reader, "SET mykey myvalue")

	// Case 1: key has no expiration -> should not set
	resp := sendCmd(t, conn, reader, "EXPIRE mykey 10 XX")
	if i, ok := resp.(parser.Integer); !ok || i != 0 {
		t.Fatalf("expected 0, got %v", resp)
	}

	// Case 2: key already has expiration -> should set
	resp2 := sendCmd(t, conn, reader, "EXPIRE mykey 20 NX")
	if i, ok := resp2.(parser.Integer); !ok || i != 1 {
		t.Fatalf("expected 1, got %v", resp2)
	}
}

func TestExpireGT(t *testing.T) {
	srv := startTestServer(t)
	defer srv.Close()

	conn, _ := net.Dial("tcp", srv.Addr())
	defer conn.Close()
	reader := bufio.NewReader(conn)

	sendCmd(t, conn, reader, "SET mykey myvalue")

	// Case 0: key has no expiration -> set to 10 seconds
	keyset := sendCmd(t, conn, reader, "EXPIRE mykey 10")
	if i, ok := keyset.(parser.Integer); !ok || i != 1 {
		t.Fatalf("expected 1, got %v", keyset)
	}

	// Case 1: set again with higher expiration -> should set
	resp := sendCmd(t, conn, reader, "EXPIRE mykey 15 GT")
	if i, ok := resp.(parser.Integer); !ok || i != 1 {
		t.Fatalf("expected 1, got %v", resp)
	}

	// Case 2: set again with Lower Expiration -> should not  set
	resp2 := sendCmd(t, conn, reader, "EXPIRE mykey 5 GT")
	if i, ok := resp2.(parser.Integer); !ok || i != 0 {
		t.Fatalf("expected 0, got %v", resp2)
	}
}

func TestExpireLT(t *testing.T) {
	srv := startTestServer(t)
	defer srv.Close()

	conn, _ := net.Dial("tcp", srv.Addr())
	defer conn.Close()
	reader := bufio.NewReader(conn)

	sendCmd(t, conn, reader, "SET mykey myvalue")

	// Case 0: key has no expiration -> set to 10 seconds
	keyset := sendCmd(t, conn, reader, "EXPIRE mykey 10")
	if i, ok := keyset.(parser.Integer); !ok || i != 1 {
		t.Fatalf("expected 1, got %v", keyset)
	}

	// Case 1: set again with higher expiration -> should not set
	resp := sendCmd(t, conn, reader, "EXPIRE mykey 15 LT")
	if i, ok := resp.(parser.Integer); !ok || i != 0 {
		t.Fatalf("expected 0, got %v", resp)
	}

	// Case 2: set again with Lower Expiration -> should set
	resp2 := sendCmd(t, conn, reader, "EXPIRE mykey 5 LT")
	if i, ok := resp2.(parser.Integer); !ok || i != 1 {
		t.Fatalf("expected 1, got %v", resp2)
	}
}

