package parser

import (
	"bufio"
	"strings"
	"testing"
	"reflect"
)

func TestDeserializeSimpleString(t *testing.T) {
	input := "+OK\r\n"
	r := bufio.NewReader(strings.NewReader(input))
	value, err := Deserialize(r)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if str, ok := value.(SimpleString); !ok || str != "OK" {
		t.Errorf("Expected SimpleString 'OK', got %v", value)
	}
}

func TestDeserializeError(t *testing.T) {
	input := "-ERR something went wrong\r\n"
	r := bufio.NewReader(strings.NewReader(input))
	value, err := Deserialize(r)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if e, ok := value.(Error); !ok || e != "ERR something went wrong" {
		t.Errorf("Expected Error 'ERR something went wrong', got %v", value)
	}
}

func TestDeserializeInteger(t *testing.T) {
	input := ":123\r\n"
	r := bufio.NewReader(strings.NewReader(input))
	value, err := Deserialize(r)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if num, ok := value.(Integer); !ok || num != 123 {
		t.Errorf("Expected Integer 123, got %v", value)
	}
}

func TestDeserializeBulkString(t *testing.T) {
	input := "$5\r\nhello\r\n"
	r := bufio.NewReader(strings.NewReader(input))
	value, err := Deserialize(r)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if bs, ok := value.(BulkString); !ok || string(bs) != "hello" {
		t.Errorf("Expected BulkString 'hello', got %v", value)
	}
}

func TestDeserializeNullBulkString(t *testing.T) {
	input := "$-1\r\n"
	r := bufio.NewReader(strings.NewReader(input))
	value, err := Deserialize(r)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if bs, ok := value.(BulkString); !ok || string(bs) != "" {
		t.Errorf("Expected Null BulkString, got %v", value)
	}
}

func TestDeserializeArray(t *testing.T) {
	input := "*3\r\n:1\r\n:2\r\n:3\r\n"
	r := bufio.NewReader(strings.NewReader(input))
	value, err := Deserialize(r)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := Array{Integer(1), Integer(2), Integer(3)}
	if !reflect.DeepEqual(value, expected) {
		t.Errorf("Expected Array %v, got %v", expected, value)
	}
}

func TestDeserializeNestedArray(t *testing.T) {
	input := "*2\r\n*2\r\n:1\r\n:2\r\n*2\r\n:3\r\n:4\r\n"
	r := bufio.NewReader(strings.NewReader(input))
	value, err := Deserialize(r)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := Array{
		Array{Integer(1), Integer(2)},
		Array{Integer(3), Integer(4)},
	}
	if !reflect.DeepEqual(value, expected) {
		t.Errorf("Expected Nested Array %v, got %v", expected, value)
	}
}

func TestDeserializeInvalidPrefix(t *testing.T) {
	input := "?invalid\r\n"
	r := bufio.NewReader(strings.NewReader(input))
	_, err := Deserialize(r)
	if err == nil {
		t.Errorf("Expected error for invalid prefix, got nil")
	}
}

func TestDeserializeIncompleteBulkString(t *testing.T) {
	input := "$5\r\nhi\r\n"
	r := bufio.NewReader(strings.NewReader(input))
	_, err := Deserialize(r)
	if err == nil {
		t.Errorf("Expected error for incomplete bulk string, got nil")
	}
}
