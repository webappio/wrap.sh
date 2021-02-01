package wrap

import (
	"reflect"
	"testing"
)

func assertEqual(t *testing.T, varName string, expected interface{}, got interface{}) {
	if expected == got {
		return
	}
	t.Fatalf("Expected %v \"%v\", got \"%v\"", varName, expected, got)
}

func isNil(got interface{}) bool {
	return got == nil || reflect.ValueOf(got).Kind() == reflect.Ptr && reflect.ValueOf(got).IsNil()
}

func assertNil(t *testing.T, varName string, got interface{}) {
	if isNil(got) {
		return
	}
	t.Fatalf("Expected %v to be nil, got \"%v\"", varName, got)
}

func assertNotNil(t *testing.T, varName string, got interface{}) {
	if !isNil(got) {
		return
	}
	t.Fatalf("Expected %v to not be nil", varName)
}

func newBlankTestClient() *Client {
	return &Client{}
}
