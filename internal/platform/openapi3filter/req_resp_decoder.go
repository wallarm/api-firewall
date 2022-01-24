package openapi3filter

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	strconvUtils "github.com/savsgio/gotils/strconv"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"github.com/wallarm/api-firewall/internal/platform/openapi3"
)

// ParseErrorKind describes a kind of ParseError.
// The type simplifies comparison of errors.
type ParseErrorKind int

const (
	// KindOther describes an untyped parsing error.
	KindOther ParseErrorKind = iota
	// KindUnsupportedFormat describes an error that happens when a value has an unsupported format.
	KindUnsupportedFormat
	// KindInvalidFormat describes an error that happens when a value does not conform a format
	// that is required by a serialization method.
	KindInvalidFormat
)

// ParseError describes errors which happens while parse operation's parameters, requestBody, or response.
type ParseError struct {
	Kind   ParseErrorKind
	Value  interface{}
	Reason string
	Cause  error

	path []interface{}
}

func (e *ParseError) Error() string {
	var msg []string
	if p := e.Path(); len(p) > 0 {
		var arr []string
		for _, v := range p {
			arr = append(arr, fmt.Sprintf("%v", v))
		}
		msg = append(msg, fmt.Sprintf("path %v", strings.Join(arr, ".")))
	}
	msg = append(msg, e.innerError())
	return strings.Join(msg, ": ")
}

func (e *ParseError) innerError() string {
	var msg []string
	if e.Value != nil {
		msg = append(msg, fmt.Sprintf("value %v", e.Value))
	}
	if e.Reason != "" {
		msg = append(msg, e.Reason)
	}
	if e.Cause != nil {
		if v, ok := e.Cause.(*ParseError); ok {
			msg = append(msg, v.innerError())
		} else {
			msg = append(msg, e.Cause.Error())
		}
	}
	return strings.Join(msg, ": ")
}

// RootCause returns a root cause of ParseError.
func (e *ParseError) RootCause() error {
	if v, ok := e.Cause.(*ParseError); ok {
		return v.RootCause()
	}
	return e.Cause
}

// Path returns a path to the root cause.
func (e *ParseError) Path() []interface{} {
	var path []interface{}
	if v, ok := e.Cause.(*ParseError); ok {
		p := v.Path()
		if len(p) > 0 {
			path = append(path, p...)
		}
	}
	if len(e.path) > 0 {
		path = append(path, e.path...)
	}
	return path
}

func invalidSerializationMethodErr(sm *openapi3.SerializationMethod) error {
	return fmt.Errorf("invalid serialization method: style=%q, explode=%v", sm.Style, sm.Explode)
}

// Decodes a parameter defined via the content property as an object. It uses
// the user specified decoder, or our build-in decoder for application/json
func decodeContentParameter(param *openapi3.Parameter, input *RequestValidationInput) (
	value interface{}, schema *openapi3.Schema, err error) {

	var paramValues []string
	var found bool
	switch param.In {
	case openapi3.ParameterInPath:
		paramValue := input.RequestCtx.UserValue(param.Name)
		if paramValue != nil {
			paramValues = []string{paramValue.(string)}
		}
	case openapi3.ParameterInQuery:
		if paramByteValues := input.GetQueryParams().PeekMulti(param.Name); len(paramByteValues) > 0 {
			paramValues = make([]string, len(paramByteValues))
			for i, value := range paramByteValues {
				paramValues[i] = strconvUtils.B2S(value)
			}
			found = true
		}
	case openapi3.ParameterInHeader:
		if paramValue := strconvUtils.B2S(input.RequestCtx.Request.Header.Peek(http.CanonicalHeaderKey(param.Name))); paramValue != "" {
			paramValues = []string{paramValue}
			found = true
		}
	case openapi3.ParameterInCookie:
		//var cookie *http.Cookie
		cookie := input.RequestCtx.Request.Header.Cookie(param.Name)
		if cookie == nil {
			found = false
		} else {
			paramValues = []string{strconvUtils.B2S(cookie)}
			found = true
		}
	default:
		err = fmt.Errorf("unsupported parameter.in: %q", param.In)
		return
	}

	if !found {
		if param.Required {
			err = fmt.Errorf("parameter %q is required, but missing", param.Name)
		}
		return
	}

	decoder := input.ParamDecoder
	if decoder == nil {
		decoder = defaultContentParameterDecoder
	}

	value, schema, err = decoder(param, paramValues)
	return
}

func defaultContentParameterDecoder(param *openapi3.Parameter, values []string) (
	outValue interface{}, outSchema *openapi3.Schema, err error) {
	// Only query parameters can have multiple values.
	if len(values) > 1 && param.In != openapi3.ParameterInQuery {
		err = fmt.Errorf("%s parameter %q cannot have multiple values", param.In, param.Name)
		return
	}

	content := param.Content
	if content == nil {
		err = fmt.Errorf("parameter %q expected to have content", param.Name)
		return
	}

	// We only know how to decode a parameter if it has one content, application/json
	if len(content) != 1 {
		err = fmt.Errorf("multiple content types for parameter %q", param.Name)
		return
	}

	mt := content.Get("application/json")
	if mt == nil {
		err = fmt.Errorf("parameter %q has no content schema", param.Name)
		return
	}
	outSchema = mt.Schema.Value

	if len(values) == 1 {
		if err = json.Unmarshal([]byte(values[0]), &outValue); err != nil {
			err = fmt.Errorf("error unmarshaling parameter %q", param.Name)
			return
		}
	} else {
		outArray := make([]interface{}, 0, len(values))
		for _, v := range values {
			var item interface{}
			if err = json.Unmarshal([]byte(v), &item); err != nil {
				err = fmt.Errorf("error unmarshaling parameter %q", param.Name)
				return
			}
			outArray = append(outArray, item)
		}
		outValue = outArray
	}
	return
}

type valueDecoder interface {
	DecodePrimitive(param string, sm *openapi3.SerializationMethod, schema *openapi3.SchemaRef) (interface{}, error)
	DecodeArray(param string, sm *openapi3.SerializationMethod, schema *openapi3.SchemaRef) ([]interface{}, error)
	DecodeObject(param string, sm *openapi3.SerializationMethod, schema *openapi3.SchemaRef) (map[string]interface{}, error)
}

// decodeStyledParameter returns a value of an operation's parameter from HTTP request for
// parameters defined using the style format.
// The function returns ParseError when HTTP request contains an invalid value of a parameter.
func decodeStyledParameter(param *openapi3.Parameter, input *RequestValidationInput) (interface{}, error) {
	sm, err := param.SerializationMethod()
	if err != nil {
		return nil, err
	}

	var dec valueDecoder
	switch param.In {
	case openapi3.ParameterInPath:
		if len(input.PathParams) == 0 {
			return nil, nil
		}
		dec = &pathParamDecoder{pathParams: input.PathParams}
	case openapi3.ParameterInQuery:
		if input.GetQueryParams().Len() == 0 {
			return nil, nil
		}
		dec = &urlValuesDecoder{values: input.GetQueryParams()}
	case openapi3.ParameterInHeader:
		dec = &headerParamDecoder{header: &input.RequestCtx.Request.Header}
	case openapi3.ParameterInCookie:
		dec = &cookieParamDecoder{req: input.RequestCtx}
	default:
		return nil, fmt.Errorf("unsupported parameter's 'in': %s", param.In)
	}

	return decodeValue(dec, param.Name, sm, param.Schema, param.Required)
}

func decodeValue(dec valueDecoder, param string, sm *openapi3.SerializationMethod, schema *openapi3.SchemaRef, required bool) (interface{}, error) {
	var decodeFn func(param string, sm *openapi3.SerializationMethod, schema *openapi3.SchemaRef) (interface{}, error)

	if len(schema.Value.AllOf) > 0 {
		var value interface{}
		var err error
		for _, sr := range schema.Value.AllOf {
			value, err = decodeValue(dec, param, sm, sr, required)
			if value == nil || err != nil {
				break
			}
		}
		return value, err
	}

	if len(schema.Value.AnyOf) > 0 {
		for _, sr := range schema.Value.AnyOf {
			value, _ := decodeValue(dec, param, sm, sr, required)
			if value != nil {
				return value, nil
			}
		}
		if required {
			return nil, fmt.Errorf("decoding anyOf for parameter %q failed", param)
		}
		return nil, nil
	}

	if len(schema.Value.OneOf) > 0 {
		isMatched := 0
		var value interface{}
		for _, sr := range schema.Value.OneOf {
			v, _ := decodeValue(dec, param, sm, sr, required)
			if v != nil {
				value = v
				isMatched++
			}
		}
		if isMatched == 1 {
			return value, nil
		} else if isMatched > 1 {
			return nil, fmt.Errorf("decoding oneOf failed: %d schemas matched", isMatched)
		}
		if required {
			return nil, fmt.Errorf("decoding oneOf failed: %q is required", param)
		}
		return nil, nil
	}

	if schema.Value.Not != nil {
		// TODO(decode not): handle decoding "not" JSON Schema
		return nil, errors.New("not implemented: decoding 'not'")
	}

	if schema.Value.Type != "" {
		switch schema.Value.Type {
		case "array":
			decodeFn = func(param string, sm *openapi3.SerializationMethod, schema *openapi3.SchemaRef) (interface{}, error) {
				return dec.DecodeArray(param, sm, schema)
			}
		case "object":
			decodeFn = func(param string, sm *openapi3.SerializationMethod, schema *openapi3.SchemaRef) (interface{}, error) {
				return dec.DecodeObject(param, sm, schema)
			}
		default:
			decodeFn = dec.DecodePrimitive
		}
		return decodeFn(param, sm, schema)
	}

	return nil, nil
}

// pathParamDecoder decodes values of path parameters.
type pathParamDecoder struct {
	pathParams map[string]string
}

func (d *pathParamDecoder) DecodePrimitive(param string, sm *openapi3.SerializationMethod, schema *openapi3.SchemaRef) (interface{}, error) {
	var prefix string
	switch sm.Style {
	case "simple":
		// A prefix is empty for style "simple".
	case "label":
		prefix = "."
	case "matrix":
		prefix = ";" + param + "="
	default:
		return nil, invalidSerializationMethodErr(sm)
	}

	if d.pathParams == nil {
		// HTTP request does not contains a value of the target path parameter.
		return nil, nil
	}
	raw, ok := d.pathParams[param]
	if !ok || raw == "" {
		// HTTP request does not contains a value of the target path parameter.
		return nil, nil
	}
	src, err := cutPrefix(raw, prefix)
	if err != nil {
		return nil, err
	}
	return parsePrimitive(src, schema)
}

func (d *pathParamDecoder) DecodeArray(param string, sm *openapi3.SerializationMethod, schema *openapi3.SchemaRef) ([]interface{}, error) {
	var prefix, delim string
	switch {
	case sm.Style == "simple":
		delim = ","
	case sm.Style == "label" && !sm.Explode:
		prefix = "."
		delim = ","
	case sm.Style == "label" && sm.Explode:
		prefix = "."
		delim = "."
	case sm.Style == "matrix" && !sm.Explode:
		prefix = ";" + param + "="
		delim = ","
	case sm.Style == "matrix" && sm.Explode:
		prefix = ";" + param + "="
		delim = ";" + param + "="
	default:
		return nil, invalidSerializationMethodErr(sm)
	}

	if d.pathParams == nil {
		// HTTP request does not contains a value of the target path parameter.
		return nil, nil
	}
	raw, ok := d.pathParams[param]
	if !ok || raw == "" {
		// HTTP request does not contains a value of the target path parameter.
		return nil, nil
	}
	src, err := cutPrefix(raw, prefix)
	if err != nil {
		return nil, err
	}
	return parseArray(strings.Split(src, delim), schema)
}

func (d *pathParamDecoder) DecodeObject(param string, sm *openapi3.SerializationMethod, schema *openapi3.SchemaRef) (map[string]interface{}, error) {
	var prefix, propsDelim, valueDelim string
	switch {
	case sm.Style == "simple" && !sm.Explode:
		propsDelim = ","
		valueDelim = ","
	case sm.Style == "simple" && sm.Explode:
		propsDelim = ","
		valueDelim = "="
	case sm.Style == "label" && !sm.Explode:
		prefix = "."
		propsDelim = ","
		valueDelim = ","
	case sm.Style == "label" && sm.Explode:
		prefix = "."
		propsDelim = "."
		valueDelim = "="
	case sm.Style == "matrix" && !sm.Explode:
		prefix = ";" + param + "="
		propsDelim = ","
		valueDelim = ","
	case sm.Style == "matrix" && sm.Explode:
		prefix = ";"
		propsDelim = ";"
		valueDelim = "="
	default:
		return nil, invalidSerializationMethodErr(sm)
	}

	if d.pathParams == nil {
		// HTTP request does not contains a value of the target path parameter.
		return nil, nil
	}
	raw, ok := d.pathParams[param]
	if !ok || raw == "" {
		// HTTP request does not contains a value of the target path parameter.
		return nil, nil
	}
	src, err := cutPrefix(raw, prefix)
	if err != nil {
		return nil, err
	}
	props, err := propsFromString(src, propsDelim, valueDelim)
	if err != nil {
		return nil, err
	}
	return makeObject(props, schema)
}

// cutPrefix validates that a raw value of a path parameter has the specified prefix,
// and returns a raw value without the prefix.
func cutPrefix(raw, prefix string) (string, error) {
	if prefix == "" {
		return raw, nil
	}
	if len(raw) < len(prefix) || raw[:len(prefix)] != prefix {
		return "", &ParseError{
			Kind:   KindInvalidFormat,
			Value:  raw,
			Reason: fmt.Sprintf("a value must be prefixed with %q", prefix),
		}
	}
	return raw[len(prefix):], nil
}

// urlValuesDecoder decodes values of query parameters.
type urlValuesDecoder struct {
	values *fasthttp.Args
}

func (d *urlValuesDecoder) DecodePrimitive(param string, sm *openapi3.SerializationMethod, schema *openapi3.SchemaRef) (interface{}, error) {
	if sm.Style != "form" {
		return nil, invalidSerializationMethodErr(sm)
	}

	values := d.values.Peek(param)
	if values == nil {
		// HTTP request does not contain a value of the target query parameter.
		return nil, nil
	}
	return parsePrimitive(strconvUtils.B2S(values), schema)
}

func (d *urlValuesDecoder) DecodeArray(param string, sm *openapi3.SerializationMethod, schema *openapi3.SchemaRef) ([]interface{}, error) {
	if sm.Style == "deepObject" {
		return nil, invalidSerializationMethodErr(sm)
	}

	bvalues := d.values.PeekMulti(param)
	if len(bvalues) == 0 {
		// HTTP request does not contain a value of the target query parameter.
		return nil, nil
	}
	if !sm.Explode {
		var delim string
		switch sm.Style {
		case "form":
			delim = ","
		case "spaceDelimited":
			delim = " "
		case "pipeDelimited":
			delim = "|"
		}
		return parseArray(strings.Split(strconvUtils.B2S(bvalues[0]), delim), schema)
	}
	values := make([]string, len(bvalues))
	for i, value := range bvalues {
		values[i] = strconvUtils.B2S(value)
	}
	//return parseArray([]string{strconvUtils.B2S(values)}, schema)
	return parseArray(values, schema)
}

func (d *urlValuesDecoder) DecodeObject(param string, sm *openapi3.SerializationMethod, schema *openapi3.SchemaRef) (map[string]interface{}, error) {
	var propsFn func(args *fasthttp.Args) (map[string]string, error)
	switch sm.Style {
	case "form":
		propsFn = func(params *fasthttp.Args) (map[string]string, error) {
			if params == nil {
				// HTTP request does not contain query parameters.
				return nil, nil
			}
			if sm.Explode {
				props := make(map[string]string)
				params.VisitAll(func(key []byte, value []byte) {
					props[strconvUtils.B2S(key)] = strconvUtils.B2S(value)
				})

				return props, nil
			}
			value := params.Peek(param)
			if value == nil {
				// HTTP request does not contain a value of the target query parameter.
				return nil, nil
			}
			return propsFromString(strconvUtils.B2S(value), ",", ",")
		}
	case "deepObject":
		propsFn = func(params *fasthttp.Args) (map[string]string, error) {
			props := make(map[string]string)
			params.VisitAll(func(key []byte, value []byte) {
				keyS := strconvUtils.B2S(key)
				valueS := strconvUtils.B2S(value)
				//props[] = strconvUtils.B2S(value)
				groups := regexp.MustCompile(fmt.Sprintf("%s\\[(.+?)\\]", param)).FindAllStringSubmatch(keyS, -1)
				if len(groups) > 0 {
					// A query parameter's name does not match the required format, so skip it.
					props[groups[0][1]] = valueS
				}
			})
			if len(props) == 0 {
				// HTTP request does not contain query parameters encoded by rules of style "deepObject".
				return nil, nil
			}
			return props, nil
		}
	default:
		return nil, invalidSerializationMethodErr(sm)
	}

	props, err := propsFn(d.values)
	if err != nil {
		return nil, err
	}
	if props == nil {
		return nil, nil
	}
	return makeObject(props, schema)
}

// headerParamDecoder decodes values of header parameters.
type headerParamDecoder struct {
	header *fasthttp.RequestHeader
}

func (d *headerParamDecoder) DecodePrimitive(param string, sm *openapi3.SerializationMethod, schema *openapi3.SchemaRef) (interface{}, error) {
	if sm.Style != "simple" {
		return nil, invalidSerializationMethodErr(sm)
	}

	raw := d.header.Peek(http.CanonicalHeaderKey(param))
	return parsePrimitive(strconvUtils.B2S(raw), schema)
}

func (d *headerParamDecoder) DecodeArray(param string, sm *openapi3.SerializationMethod, schema *openapi3.SchemaRef) ([]interface{}, error) {
	if sm.Style != "simple" {
		return nil, invalidSerializationMethodErr(sm)
	}

	raw := d.header.Peek(http.CanonicalHeaderKey(param))
	if raw == nil {
		// HTTP request does not contains a corresponding header
		return nil, nil
	}
	return parseArray(strings.Split(strconvUtils.B2S(raw), ","), schema)
}

func (d *headerParamDecoder) DecodeObject(param string, sm *openapi3.SerializationMethod, schema *openapi3.SchemaRef) (map[string]interface{}, error) {
	if sm.Style != "simple" {
		return nil, invalidSerializationMethodErr(sm)
	}
	valueDelim := ","
	if sm.Explode {
		valueDelim = "="
	}

	raw := d.header.Peek(http.CanonicalHeaderKey(param))
	if raw == nil {
		// HTTP request does not contain a corresponding header.
		return nil, nil
	}
	props, err := propsFromString(strconvUtils.B2S(raw), ",", valueDelim)
	if err != nil {
		return nil, err
	}
	return makeObject(props, schema)
}

// cookieParamDecoder decodes values of cookie parameters.
type cookieParamDecoder struct {
	req *fasthttp.RequestCtx
}

func (d *cookieParamDecoder) DecodePrimitive(param string, sm *openapi3.SerializationMethod, schema *openapi3.SchemaRef) (interface{}, error) {
	if sm.Style != "form" {
		return nil, invalidSerializationMethodErr(sm)
	}

	cookie := d.req.Request.Header.Cookie(param)
	if cookie == nil {
		// HTTP request does not contain a corresponding cookie.
		return nil, nil
	}
	return parsePrimitive(strconvUtils.B2S(cookie), schema)
}

func (d *cookieParamDecoder) DecodeArray(param string, sm *openapi3.SerializationMethod, schema *openapi3.SchemaRef) ([]interface{}, error) {
	if sm.Style != "form" || sm.Explode {
		return nil, invalidSerializationMethodErr(sm)
	}

	cookie := d.req.Request.Header.Cookie(param)
	if cookie == nil {
		// HTTP request does not contain a corresponding cookie.
		return nil, nil
	}
	return parseArray(strings.Split(strconvUtils.B2S(cookie), ","), schema)
}

func (d *cookieParamDecoder) DecodeObject(param string, sm *openapi3.SerializationMethod, schema *openapi3.SchemaRef) (map[string]interface{}, error) {
	if sm.Style != "form" || sm.Explode {
		return nil, invalidSerializationMethodErr(sm)
	}

	cookie := d.req.Request.Header.Cookie(param)
	if cookie == nil {
		// HTTP request does not contain a corresponding cookie.
		return nil, nil
	}
	props, err := propsFromString(strconvUtils.B2S(cookie), ",", ",")
	if err != nil {
		return nil, err
	}
	return makeObject(props, schema)
}

// propsFromString returns a properties map that is created by splitting a source string by propDelim and valueDelim.
// The source string must have a valid format: pairs <propName><valueDelim><propValue> separated by <propDelim>.
// The function returns an error when the source string has an invalid format.
func propsFromString(src, propDelim, valueDelim string) (map[string]string, error) {
	props := make(map[string]string)
	pairs := strings.Split(src, propDelim)

	// When propDelim and valueDelim is equal the source string follow the next rule:
	// every even item of pairs is a properies's name, and the subsequent odd item is a property's value.
	if propDelim == valueDelim {
		// Taking into account the rule above, a valid source string must be splitted by propDelim
		// to an array with an even number of items.
		if len(pairs)%2 != 0 {
			return nil, &ParseError{
				Kind:   KindInvalidFormat,
				Value:  src,
				Reason: fmt.Sprintf("a value must be a list of object's properties in format \"name%svalue\" separated by %s", valueDelim, propDelim),
			}
		}
		for i := 0; i < len(pairs)/2; i++ {
			props[pairs[i*2]] = pairs[i*2+1]
		}
		return props, nil
	}

	// When propDelim and valueDelim is not equal the source string follow the next rule:
	// every item of pairs is a string that follows format <propName><valueDelim><propValue>.
	for _, pair := range pairs {
		prop := strings.Split(pair, valueDelim)
		if len(prop) != 2 {
			return nil, &ParseError{
				Kind:   KindInvalidFormat,
				Value:  src,
				Reason: fmt.Sprintf("a value must be a list of object's properties in format \"name%svalue\" separated by %s", valueDelim, propDelim),
			}
		}
		props[prop[0]] = prop[1]
	}
	return props, nil
}

// makeObject returns an object that contains properties from props.
// A value of every property is parsed as a primitive value.
// The function returns an error when an error happened while parse object's properties.
func makeObject(props map[string]string, schema *openapi3.SchemaRef) (map[string]interface{}, error) {
	obj := make(map[string]interface{})
	for propName, propSchema := range schema.Value.Properties {
		value, err := parsePrimitive(props[propName], propSchema)
		if err != nil {
			if v, ok := err.(*ParseError); ok {
				return nil, &ParseError{path: []interface{}{propName}, Cause: v}
			}
			return nil, fmt.Errorf("property %q: %s", propName, err)
		}
		obj[propName] = value
	}
	return obj, nil
}

// parseArray returns an array that contains items from a raw array.
// Every item is parsed as a primitive value.
// The function returns an error when an error happened while parse array's items.
func parseArray(raw []string, schemaRef *openapi3.SchemaRef) ([]interface{}, error) {
	var value []interface{}
	for i, v := range raw {
		item, err := parsePrimitive(v, schemaRef.Value.Items)
		if err != nil {
			if v, ok := err.(*ParseError); ok {
				return nil, &ParseError{path: []interface{}{i}, Cause: v}
			}
			return nil, fmt.Errorf("item %d: %s", i, err)
		}
		value = append(value, item)
	}
	return value, nil
}

// parsePrimitive returns a value that is created by parsing a source string to a primitive type
// that is specified by a schema. The function returns nil when the source string is empty.
// The function panics when a schema has a non primitive type.
func parsePrimitive(raw string, schema *openapi3.SchemaRef) (interface{}, error) {
	if raw == "" {
		return nil, nil
	}
	switch schema.Value.Type {
	case "integer":
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, &ParseError{Kind: KindInvalidFormat, Value: raw, Reason: "an invalid integer", Cause: err}
		}
		return v, nil
	case "number":
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, &ParseError{Kind: KindInvalidFormat, Value: raw, Reason: "an invalid number", Cause: err}
		}
		return v, nil
	case "boolean":
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, &ParseError{Kind: KindInvalidFormat, Value: raw, Reason: "an invalid number", Cause: err}
		}
		return v, nil
	case "string":
		return raw, nil
	default:
		panic(fmt.Sprintf("schema has non primitive type %q", schema.Value.Type))
	}
}

// EncodingFn is a function that returns an encoding of a request body's part.
type EncodingFn func(partName string) *openapi3.Encoding

// BodyDecoder is an interface to decode a body of a request or response.
// An implementation must return a value that is a primitive, []interface{}, or map[string]interface{}.
type BodyDecoder func(io.Reader, []byte, string, *openapi3.SchemaRef, EncodingFn, *fastjson.Parser) (interface{}, error)

// bodyDecoders contains decoders for supported content types of a body.
// By default, there is content type "application/json" is supported only.
var bodyDecoders = make(map[string]BodyDecoder)

// RegisterBodyDecoder registers a request body's decoder for a content type.
//
// If a decoder for the specified content type already exists, the function replaces
// it with the specified decoder.
func RegisterBodyDecoder(contentType string, decoder BodyDecoder) {
	if contentType == "" {
		panic("contentType is empty")
	}
	if decoder == nil {
		panic("decoder is not defined")
	}
	bodyDecoders[contentType] = decoder
}

// UnregisterBodyDecoder dissociates a body decoder from a content type.
//
// Decoding this content type will result in an error.
func UnregisterBodyDecoder(contentType string) {
	if contentType == "" {
		panic("contentType is empty")
	}
	delete(bodyDecoders, contentType)
}

var headerCT = http.CanonicalHeaderKey("Content-Type")

const prefixUnsupportedCT = "unsupported content type"

// decodeBody returns a decoded body.
// The function returns ParseError when a body is invalid.
func decodeBody(body io.Reader, bodyBytes []byte, contentType string, schema *openapi3.SchemaRef, encFn EncodingFn, parser *fastjson.Parser) (interface{}, error) {
	if contentType == "" {
		if _, ok := body.(*multipart.Part); ok {
			contentType = "text/plain"
		}
	}
	mediaType := parseMediaType(contentType)
	decoder, ok := bodyDecoders[mediaType]
	if !ok {
		// use default decoder which returns empty object
		decoder = defaultEmptyBodyDecoder
	}
	value, err := decoder(body, bodyBytes, contentType, schema, encFn, parser)
	if err != nil {
		return nil, err
	}
	return value, nil
}

func init() {
	RegisterBodyDecoder("text/plain", plainBodyDecoder)
	RegisterBodyDecoder("application/json", jsonBodyDecoder)
	RegisterBodyDecoder("application/problem+json", jsonBodyDecoder)
	RegisterBodyDecoder("application/x-www-form-urlencoded", urlencodedBodyDecoder)
	RegisterBodyDecoder("multipart/form-data", multipartBodyDecoder)
	RegisterBodyDecoder("application/octet-stream", FileBodyDecoder)
}

func defaultEmptyBodyDecoder(body io.Reader, bodyBytes []byte, contentType string, schema *openapi3.SchemaRef, encFn EncodingFn, parser *fastjson.Parser) (interface{}, error) {
	if schema != nil {
		switch schema.Value.Type {
		case "object":
			obj := make(map[string]interface{})
			return obj, nil
		case "null":
			return nil, nil
		case "integer", "number", "boolean", "array":
			return nil, &ParseError{Kind: KindUnsupportedFormat, Cause: fmt.Errorf("schema not fully supported: %v", contentType)}
		}
	}
	if bodyBytes != nil {
		return string(bodyBytes), nil
	}
	if body != nil {
		data, err := ioutil.ReadAll(body)
		if err != nil {
			return nil, &ParseError{Kind: KindInvalidFormat, Cause: err}
		}
		return string(data), nil
	}

	return "", nil
}

func plainBodyDecoder(body io.Reader, bodyBytes []byte, contentType string, schema *openapi3.SchemaRef, encFn EncodingFn, parser *fastjson.Parser) (interface{}, error) {
	if bodyBytes != nil {
		return string(bodyBytes), nil
	}
	data, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, &ParseError{Kind: KindInvalidFormat, Cause: err}
	}
	return string(data), nil
}

func jsonBodyDecoder(body io.Reader, bodyBytes []byte, contentType string, schema *openapi3.SchemaRef, encFn EncodingFn, parser *fastjson.Parser) (interface{}, error) {
	var data []byte
	var err error

	if bodyBytes != nil {
		data = bodyBytes
	} else {
		data, err = ioutil.ReadAll(body)
		if err != nil {
			return nil, &ParseError{Kind: KindInvalidFormat, Cause: err}
		}
	}

	parsedDoc, err := parser.ParseBytes(data)
	if err != nil {
		return nil, &ParseError{Kind: KindInvalidFormat, Cause: err}
	}

	return parsedDoc, nil
}

func urlencodedBodyDecoder(body io.Reader, bodyBytes []byte, contentType string, schema *openapi3.SchemaRef, encFn EncodingFn, parser *fastjson.Parser) (interface{}, error) {
	// Validate schema of request body.
	// By the OpenAPI 3 specification request body's schema must have type "object".
	// Properties of the schema describes individual parts of request body.
	if schema.Value.Type != "object" {
		return nil, errors.New("unsupported schema of request body")
	}
	for propName, propSchema := range schema.Value.Properties {
		switch propSchema.Value.Type {
		case "object":
			return nil, fmt.Errorf("unsupported schema of request body's property %q", propName)
		case "array":
			items := propSchema.Value.Items.Value
			if items.Type != "string" && items.Type != "integer" && items.Type != "number" && items.Type != "boolean" {
				return nil, fmt.Errorf("unsupported schema of request body's property %q", propName)
			}
		}
	}

	// Parse form.
	b, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}

	var args fasthttp.Args
	args.ParseBytes(b)

	// Make an object value from form values.
	obj := make(map[string]interface{})
	dec := &urlValuesDecoder{values: &args}
	for name, prop := range schema.Value.Properties {
		var (
			value interface{}
			enc   *openapi3.Encoding
		)
		if encFn != nil {
			enc = encFn(name)
		}
		sm := enc.SerializationMethod()

		value, err := decodeValue(dec, name, sm, prop, false)
		if err != nil {
			return nil, err
		}
		obj[name] = value
	}

	return obj, nil
}

func multipartBodyDecoder(body io.Reader, bodyBytes []byte, contentType string, schema *openapi3.SchemaRef, encFn EncodingFn, parser *fastjson.Parser) (interface{}, error) {
	if schema.Value.Type != "object" {
		return nil, errors.New("unsupported schema of request body")
	}

	// Parse form.
	values := make(map[string][]interface{})
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, err
	}
	mr := multipart.NewReader(body, params["boundary"])
	for {
		var part *multipart.Part
		if part, err = mr.NextPart(); err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		var (
			name = part.FormName()
			enc  *openapi3.Encoding
		)
		if encFn != nil {
			enc = encFn(name)
		}
		subEncFn := func(string) *openapi3.Encoding { return enc }
		// If the property's schema has type "array" it is means that the form contains a few parts with the same name.
		// Every such part has a type that is defined by an items schema in the property's schema.
		var valueSchema *openapi3.SchemaRef
		var exists bool
		valueSchema, exists = schema.Value.Properties[name]
		if !exists {
			anyProperties := schema.Value.AdditionalPropertiesAllowed
			if anyProperties != nil {
				switch *anyProperties {
				case true:
					//additionalProperties: true
					continue
				default:
					//additionalProperties: false
					return nil, &ParseError{Kind: KindOther, Cause: fmt.Errorf("part %s: undefined", name)}
				}
			}
			if schema.Value.AdditionalProperties == nil {
				return nil, &ParseError{Kind: KindOther, Cause: fmt.Errorf("part %s: undefined", name)}
			}
			valueSchema, exists = schema.Value.AdditionalProperties.Value.Properties[name]
			if !exists {
				return nil, &ParseError{Kind: KindOther, Cause: fmt.Errorf("part %s: undefined", name)}
			}
		}
		if valueSchema.Value.Type == "array" {
			valueSchema = valueSchema.Value.Items
		}

		var value interface{}

		mHeaderCT := part.Header.Get(headerCT)

		var parserNew fastjson.Parser

		if value, err = decodeBody(part, nil, mHeaderCT, valueSchema, subEncFn, &parserNew); err != nil {
			if v, ok := err.(*ParseError); ok {
				return nil, &ParseError{path: []interface{}{name}, Cause: v}
			}
			return nil, fmt.Errorf("part %s: %s", name, err)
		}
		values[name] = append(values[name], value)
	}

	allTheProperties := make(map[string]*openapi3.SchemaRef)
	for k, v := range schema.Value.Properties {
		allTheProperties[k] = v
	}
	if schema.Value.AdditionalProperties != nil {
		for k, v := range schema.Value.AdditionalProperties.Value.Properties {
			allTheProperties[k] = v
		}
	}
	// Make an object value from form values.
	obj := make(map[string]interface{})
	for name, prop := range allTheProperties {
		vv := values[name]
		if len(vv) == 0 {
			continue
		}
		if prop.Value.Type == "array" {
			obj[name] = vv
		} else {
			obj[name] = vv[0]
		}
	}

	return obj, nil
}

// FileBodyDecoder is a body decoder that decodes a file body to a string.
func FileBodyDecoder(body io.Reader, bodyBytes []byte, contentType string, schema *openapi3.SchemaRef, encFn EncodingFn, parser *fastjson.Parser) (interface{}, error) {
	if bodyBytes != nil {
		return string(bodyBytes), nil
	}
	data, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}
