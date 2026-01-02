package parser

import (
	"bufio"
	"io"
	"fmt"
	"strconv"
	"strings"
	"errors"
)

func main() {
	fmt.Println("Hi")
}

type Value interface{}

type SimpleString string
type Error string
type Integer int64
type BulkString []byte
type Array []Value

func handleSimpleString(r *bufio.Reader) (Value, error) {
	line, _ := r.ReadString('\n')
	return SimpleString(strings.TrimSuffix(line, "\r\n")), nil
}

func handleError(r *bufio.Reader) (Value, error) {
	line, _ := r.ReadString('\n')
	return Error(strings.TrimSuffix(line, "\r\n")), nil
}

func handleInteger(r *bufio.Reader) (Value, error) {

	line, _ := r.ReadString('\n')
	trimmed := strings.TrimSuffix(line, "\r\n")
	
	if len(trimmed) > 0 && trimmed[0] == '+' {
		trimmed = trimmed[1:]
	}
	num64, err := strconv.ParseInt(trimmed, 10, 64)

	if err != nil {
		return nil, fmt.Errorf("cannot parse integer from bytes: %w", err)
	}
	return Integer(num64), nil
}

func handleBulkString(r *bufio.Reader) (Value, error) {
	
	line, _ := r.ReadString('\n')
	lengthStr :=  strings.TrimSpace(line)
	length, err := strconv.Atoi(lengthStr)

	if err != nil {
		return nil, fmt.Errorf("cannot parse the length delimiter for Bulk String: %w", err)
	}

	if length == -1 {
        return nil, nil
    }

	buf := make([]byte, length+2)
	_, err = io.ReadFull(r, buf)

	if err != nil {
		return nil, fmt.Errorf("cannot read from the buffer: %w", err)
	}
	content := string(buf[:length])

	return BulkString(content), nil
}

func handleArray(r *bufio.Reader) (Value, error) {

	line, _ := r.ReadString('\n')
	lengthStr :=  strings.TrimSpace(line)
	length, err := strconv.Atoi(lengthStr)

	if err != nil {
		return nil, fmt.Errorf("cannot parse the length delimiter for Array: %w", err)
	}

	returnValues := make([]Value, length)
	
	for i:=0; i < length; i++ {
		prefix, _ :=  r.ReadByte() 
		returnValues[i], err = handleCommand(prefix, r)
		if err != nil {
			return nil, fmt.Errorf("Error when parsing Array: %w", err)
		}
	}

	return Array(returnValues), nil
}

func handleInline(r *bufio.Reader, firstByte byte) (Value, error) {
    line, err := r.ReadString('\n')
    if err != nil {
        return nil, err
    }
    // prepend first byte that was already read
    line = string(firstByte) + line
    line = strings.TrimSuffix(line, "\r\n")
    tokens := strings.Fields(line)
    if len(tokens) == 0 {
        return nil, errors.New("empty inline command")
    }

	returnArr := []Value{}
	returnArr = append(returnArr, SimpleString(tokens[0]))
    // return only the command name for now
    return Array(returnArr), nil
}

func handleCommand(prefix byte, r *bufio.Reader) (Value, error) {

	switch prefix {
	case '+':
		result, _ := handleSimpleString(r)
		return result, nil

	case '-':
		result, _ := handleError(r)
		return result, nil
	
	case ':':
		result, _ := handleInteger(r)
		return result, nil

	case '$':
		result, err := handleBulkString(r)

		if err != nil {
			return nil, fmt.Errorf("Error handling Bulk String: %w", err)
		}
		return result, nil
	case '*':
		result, err := handleArray(r)
		
		if err != nil {
			return nil, fmt.Errorf("Error handling Array: %w", err)
		}
		return result, nil
	
	default:
		return handleInline(r, prefix)

	}
}

func Deserialize(r *bufio.Reader) (Value, error){

	prefix, err :=  r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("Error reading from the byte stream: %w", err)
	}
	result, err := handleCommand(prefix, r)
	if err != nil {
		return nil, fmt.Errorf("Deserializing Error: %w", err)
	}
	return result, nil

}

func Serialize(v Value) ([]byte, error) {
	switch t := v.(type) {
	case SimpleString:
		return []byte("+" + string(t) + "\r\n"), nil
	case Error:
		return []byte("-" + string(t) + "\r\n"), nil
	case Integer:
		return []byte(":" + strconv.FormatInt(int64(t), 10) + "\r\n"), nil
	case BulkString:
		if t == nil {
			return []byte("$-1\r\n"), nil
		}
		return []byte("$" + strconv.Itoa(len(t)) + "\r\n" + string(t) + "\r\n"), nil
	case Array:
		result := "*" + strconv.Itoa(len(t)) + "\r\n"
		for _, elem := range t {
			serialized, err := Serialize(elem)
			if err != nil {
				return nil, err
			}
			result += string(serialized)
		}
		return []byte(result), nil
	default:
		return nil, fmt.Errorf("unknown type %T", v)
	}
}

func SerializeFromString(s string) ([]byte, error) {

	parts := strings.Fields(s)
	values := make([]Value, len(parts))

	for i := range values {		

		values[i] = BulkString(parts[i])
	}

	serializedArray, err := Serialize(Array(values))

	if err != nil {
		return nil, fmt.Errorf("Error Serializing Array: %w", err)
	}
	return serializedArray, nil

}

func unwrap(v Value)  {
    switch val := v.(type) {
    case SimpleString:
        fmt.Println(string(val))

    case Error:
        fmt.Println(string(val))

    case Integer:
        fmt.Println(int64(val))

    case BulkString:
        fmt.Println(string([]byte(val)))

    case Array:
		for _, elem := range val {
    		unwrap(elem)
		}

    default:
        break
    }
}