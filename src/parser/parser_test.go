package parser

import (
	"bufio"
	"reflect"
	"strings"
	"testing"
	"strconv"
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

	if value != nil {
		t.Errorf("Expected nil BulkString, got %v", value)
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

func TestSerialize(t *testing.T) {
	tests := []struct {
		name     string
		input    Value
		expected []byte
		wantErr  bool
	}{
		{
			name:     "SimpleString",
			input:    SimpleString("OK"),
			expected: []byte("+OK\r\n"),
			wantErr:  false,
		},
		{
			name:     "Error",
			input:    Error("ERR something"),
			expected: []byte("-ERR something\r\n"),
			wantErr:  false,
		},
		{
			name:     "Integer",
			input:    Integer(123),
			expected: []byte(":" + strconv.FormatInt(123, 10) + "\r\n"),
			wantErr:  false,
		},
		{
			name:     "BulkString non-nil",
			input:    BulkString("hello"),
			expected: []byte("$5\r\nhello\r\n"),
			wantErr:  false,
		},
		{
			name:     "BulkString nil",
			input:    BulkString(nil),
			expected: []byte("$-1\r\n"),
			wantErr:  false,
		},
		{
			name: "Array of mixed types",
			input: Array{
				SimpleString("OK"),
				Integer(42),
				BulkString("hi"),
			},
			expected: []byte("*3\r\n+OK\r\n:42\r\n$2\r\nhi\r\n"),
			wantErr:  false,
		},
		{
			name: "Nested Array",
			input: Array{
				SimpleString("A"),
				Array{
					Integer(1),
					BulkString("nested"),
				},
			},
			expected: []byte("*2\r\n+A\r\n*2\r\n:1\r\n$6\r\nnested\r\n"),
			wantErr:  false,
		},
		{
			name:     "Unknown type",
			input:    3.14, // float64 is not supported
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Serialize(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Serialize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("Serialize() = %q, want %q", got, tt.expected)
			}
		})
	}
}
