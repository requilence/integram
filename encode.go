package integram

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"reflect"
)

var errorType = reflect.TypeOf(make([]error, 1)).Elem()
var contextType = reflect.TypeOf(&Context{})

func typeIsError(typ reflect.Type) bool {
	return typ.Implements(errorType)
}

// decode decodes a slice of bytes and scans the value into dest using the gob package.
// All types are supported except recursive data structures and functions.
func decode(reply []byte, dest interface{}) error {
	// Check the type of dest and make sure it is a pointer to something,
	// otherwise we can't set its value in any meaningful way.
	val := reflect.ValueOf(dest)
	if val.Kind() != reflect.Ptr {
		return fmt.Errorf("Argument to decode must be pointer. Got %T", dest)
	}

	// Use the gob package to decode the reply and write the result into
	// dest.
	buf := bytes.NewBuffer(reply)
	dec := gob.NewDecoder(buf)
	if err := dec.DecodeValue(val.Elem()); err != nil {
		return err
	}
	return nil
}

// encode encodes data into a slice of bytes using the gob package.
// All types are supported except recursive data structures and functions.
func encode(data interface{}) ([]byte, error) {
	if data == nil {
		return nil, nil
	}
	buf := bytes.NewBuffer([]byte{})
	enc := gob.NewEncoder(buf)
	if err := enc.Encode(data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func verifyTypeMatching(handlerFunc interface{}, args ...interface{}) error {
	// Check the type of data
	// Make sure handler is a function
	handlerType := reflect.TypeOf(handlerFunc)
	if handlerType.Kind() != reflect.Func {
		return fmt.Errorf("handler must be a function. Got %T", handlerFunc)
	}

	if handlerType.NumIn() == 0 {
		return fmt.Errorf("handler first arg must be a %s. ", contextType.String())
	}

	if handlerType.In(0) != contextType {
		return fmt.Errorf("handler first arg must be a %s. Got %s", contextType.String(), handlerType.In(0).String())
	}

	if handlerType.NumIn() != (len(args) + 1) {
		return fmt.Errorf("handler have %d args, you must call it with %d args (ommit the first %s). Instead got %d args", handlerType.NumIn(), handlerType.NumIn()-1, contextType.String(), len(args))
	}

	if handlerType.NumOut() != 1 {
		return fmt.Errorf("handler must have exactly one return value. Got %d", handlerType.NumOut())
	}
	if !typeIsError(handlerType.Out(0)) {
		return fmt.Errorf("handler must return an error. Got return value of type %s", handlerType.Out(0).String())
	}

	for argIndex, arg := range args {
		handlerArgType := handlerType.In(argIndex + 1)
		argType := reflect.TypeOf(arg)
		if handlerArgType != argType {
			return fmt.Errorf("provided data was not of the correct type.\nExpected %s, but got %s", handlerArgType.String(), argType.String())
		}

		if reflect.Zero(argType) == reflect.ValueOf(arg) {
			return fmt.Errorf("You can't send zero-valued arguments, received zero %v arg[%d]", argType.String(), argIndex)
		}
	}
	return nil
}
