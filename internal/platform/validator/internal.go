package validator

import (
	"github.com/valyala/fastjson"
	"reflect"
	"strings"
)

func parseMediaType(contentType string) string {
	i := strings.IndexByte(contentType, ';')
	if i < 0 {
		return contentType
	}
	return contentType[:i]
}

func isNilValue(value interface{}) bool {
	if value == nil {
		return true
	}
	switch reflect.TypeOf(value).Kind() {
	case reflect.Ptr, reflect.Map, reflect.Array, reflect.Chan, reflect.Slice:
		return reflect.ValueOf(value).IsNil()
	}
	return false
}

func convertToMap(v *fastjson.Value) interface{} {
	switch v.Type() {
	case fastjson.TypeObject:
		m := make(map[string]interface{})
		v.GetObject().Visit(func(k []byte, v *fastjson.Value) {
			m[string(k)] = convertToMap(v)
		})
		return m
	case fastjson.TypeArray:
		var a []interface{}
		for _, v := range v.GetArray() {
			a = append(a, convertToMap(v))
		}
		return a
	case fastjson.TypeNumber:
		return v.GetFloat64()
	case fastjson.TypeString:
		return string(v.GetStringBytes())
	case fastjson.TypeTrue, fastjson.TypeFalse:
		return v.GetBool()
	default:
		return nil
	}
}
