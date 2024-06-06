package validator

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/valyala/fastjson"
)

// parseMediaType func parses content type and returns media type and suffix
func parseMediaType(contentType string) (string, string) {

	var mtSubtype, suffix string
	mediaType := contentType

	if i := strings.IndexByte(mediaType, ';'); i >= 0 {
		mediaType = strings.TrimSpace(mediaType[:i])
	}

	if i := strings.IndexByte(mediaType, '/'); i >= 0 {
		mtSubtype = mediaType[i+1:]
	}

	if i := strings.LastIndexByte(mtSubtype, '+'); i >= 0 {
		suffix = mtSubtype[i:]
	}

	return mediaType, suffix
}

func isNilValue(value any) bool {
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
		return json.Number(v.String())
	case fastjson.TypeString:
		return string(v.GetStringBytes())
	case fastjson.TypeTrue, fastjson.TypeFalse:
		return v.GetBool()
	default:
		return nil
	}
}

// Contains returns true if v is present in the elems slice, false otherwise
func Contains[T comparable](elems []T, v T) bool {
	for _, s := range elems {
		if v == s {
			return true
		}
	}
	return false
}
