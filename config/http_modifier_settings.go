package config

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// HTTPModifierConfig holds configuration options for built-in traffic modifier
type HTTPModifierConfig struct {
	URLNegativeRegexp      HTTPURLRegexp              `json:"http-disallow-url"`
	URLRegexp              HTTPURLRegexp              `json:"http-allow-url"`
	URLRewrite             URLRewriteMap              `json:"http-rewrite-url"`
	HeaderRewrite          HeaderRewriteMap           `json:"http-rewrite-header"`
	HeaderFilters          HTTPHeaderFilters          `json:"http-allow-header"`
	HeaderNegativeFilters  HTTPHeaderFilters          `json:"http-disallow-header"`
	HeaderBasicAuthFilters HTTPHeaderBasicAuthFilters `json:"http-basic-auth-filter"`
	HeaderHashFilters      HTTPHashFilters            `json:"http-header-limiter"`
	ParamHashFilters       HTTPHashFilters            `json:"http-param-limiter"`
	Params                 HTTPParams                 `json:"http-set-param"`
	Headers                HTTPHeaders                `json:"http-set-header"`
	Methods                HTTPMethods                `json:"http-allow-method"`
}

//
// Handling of --http-allow-header, --http-disallow-header options
//
type headerFilter struct {
	Name   []byte
	Regexp *regexp.Regexp
}

// HTTPHeaderFilters holds list of headers and their regexps
type HTTPHeaderFilters []headerFilter

// String HTTPHeaderFilters to string method
func (h *HTTPHeaderFilters) String() string {
	return fmt.Sprint(*h)
}

// Set method to implement flags.Value
func (h *HTTPHeaderFilters) Set(value string) error {
	valArr := strings.SplitN(value, ":", 2)
	if len(valArr) < 2 {
		return fmt.Errorf("need both header and value, colon-delimited (ex. user_id:^169$)")
	}
	val := strings.TrimSpace(valArr[1])
	r, err := regexp.Compile(val)
	if err != nil {
		return err
	}

	*h = append(*h, headerFilter{Name: []byte(valArr[0]), Regexp: r})

	return nil
}

//
// Handling of --http-basic-auth-filter option
//
type basicAuthFilter struct {
	Regexp *regexp.Regexp
}

// HTTPHeaderBasicAuthFilters holds list of regxp to match basic Auth header values
type HTTPHeaderBasicAuthFilters []basicAuthFilter

// String HTTPHeaderBasicAuthFilters to string method
func (h *HTTPHeaderBasicAuthFilters) String() string {
	return fmt.Sprint(*h)
}

// Set method to implement flags.Value
func (h *HTTPHeaderBasicAuthFilters) Set(value string) error {
	r, err := regexp.Compile(value)
	if err != nil {
		return err
	}

	*h = append(*h, basicAuthFilter{Regexp: r})

	return nil
}

//
// Handling of --http-allow-header-hash and --http-allow-param-hash options
//
type hashFilter struct {
	Name    []byte
	Percent uint32
}

// HTTPHashFilters represents a slice of header hash filters
type HTTPHashFilters []hashFilter

// String HTTPHashFilters to string method
func (h *HTTPHashFilters) String() string {
	return fmt.Sprint(*h)
}

// Set method to implement flags.Value
func (h *HTTPHashFilters) Set(value string) error {
	valArr := strings.SplitN(value, ":", 2)
	if len(valArr) < 2 {
		return fmt.Errorf("need both header and value, colon-delimited (ex. user_id:50%%)")
	}

	f := hashFilter{Name: []byte(valArr[0])}
	val := strings.TrimSpace(valArr[1])

	switch {
	case strings.Contains(val, "%"):
		p, err := strconv.ParseInt(val[:len(val)-1], 0, 0)
		if err != nil {
			return err
		}

		f.Percent = uint32(p)
	case strings.Contains(val, "/"):
		fracArr := strings.Split(val, "/")
		num, err := strconv.ParseUint(fracArr[0], 10, 64)
		if err != nil {
			return err
		}

		den, err := strconv.ParseUint(fracArr[1], 10, 64)
		if err != nil {
			return err
		}

		f.Percent = uint32((float64(num) / float64(den)) * 100)
	default:
		return fmt.Errorf("value should be percent and contain '%%'")
	}

	*h = append(*h, f)

	return nil
}

//
// Handling of --http-set-header option
//
type httpHeader struct {
	Name  string
	Value string
}

// HTTPHeaders is a slice of headers that must appended
type HTTPHeaders []httpHeader

// String HTTPHeaders to string method
func (h *HTTPHeaders) String() string {
	return fmt.Sprint(*h)
}

// Set method to implement flags.Value
func (h *HTTPHeaders) Set(value string) error {
	v := strings.SplitN(value, ":", 2)
	if len(v) != 2 {
		return errors.New("expected `Key: Value`")
	}

	header := httpHeader{
		strings.TrimSpace(v[0]),
		strings.TrimSpace(v[1]),
	}

	*h = append(*h, header)

	return nil
}

//
// Handling of --http-set-param option
//
type httpParam struct {
	Name  []byte
	Value []byte
}

// HTTPParams filters for --http-set-param
type HTTPParams []httpParam

// String HTTPParams to string method
func (h *HTTPParams) String() string {
	return fmt.Sprint(*h)
}

// Set method to implement flags.Value
func (h *HTTPParams) Set(value string) error {
	v := strings.SplitN(value, "=", 2)
	if len(v) != 2 {
		return errors.New("expected `Key=Value`")
	}

	param := httpParam{
		[]byte(strings.TrimSpace(v[0])),
		[]byte(strings.TrimSpace(v[1])),
	}

	*h = append(*h, param)

	return nil
}

//
// Handling of --http-allow-method option
//

// HTTPMethods holds values for method allowed
type HTTPMethods [][]byte

// String HTTPMethods to string method
func (h *HTTPMethods) String() string {
	return fmt.Sprint(*h)
}

// Set method to implement flags.Value
func (h *HTTPMethods) Set(value string) error {
	*h = append(*h, []byte(value))

	return nil
}

//
// Handling of --http-rewrite-url option
//
type urlRewrite struct {
	Regexp *regexp.Regexp
	Target []byte
}

// URLRewriteMap holds regexp and data to modify URL
type URLRewriteMap []urlRewrite

// String URLRewriteMap to string method
func (r *URLRewriteMap) String() string {
	return fmt.Sprint(*r)
}

// Set method to implement flags.Value
func (r *URLRewriteMap) Set(value string) error {
	valArr := strings.SplitN(value, ":", 2)
	if len(valArr) < 2 {
		return fmt.Errorf("need both src and target, colon-delimited (ex. /a:/b)")
	}

	regularExp, err := regexp.Compile(valArr[0])
	if err != nil {
		return err
	}

	*r = append(*r, urlRewrite{Regexp: regularExp, Target: []byte(valArr[1])})

	return nil
}

//
// Handling of --http-rewrite-header option
//
type headerRewrite struct {
	Header []byte
	Regexp *regexp.Regexp
	Target []byte
}

// HeaderRewriteMap holds regexp and data to rewrite headers
type HeaderRewriteMap []headerRewrite

// String HeaderRewriteMap to string method
func (r *HeaderRewriteMap) String() string {
	return fmt.Sprint(*r)
}

// Set method to implement flags.Value
func (r *HeaderRewriteMap) Set(value string) error {
	headerArr := strings.SplitN(value, ":", 2)
	if len(headerArr) < 2 {
		return errors.New("need both header, regexp and rewrite target, " +
			"colon-delimited (ex. Header: regexp,target)")
	}

	header := headerArr[0]
	valArr := strings.SplitN(strings.TrimSpace(headerArr[1]), ",", 2)

	if len(valArr) < 2 {
		return errors.New("need both header, regexp and rewrite target, " +
			"colon-delimited (ex. Header: regexp,target)")
	}

	regularExp, err := regexp.Compile(valArr[0])
	if err != nil {
		return err
	}

	*r = append(*r, headerRewrite{Header: []byte(header), Regexp: regularExp, Target: []byte(valArr[1])})

	return nil
}

//
// Handling of --http-allow-url option
//
type urlRegexp struct {
	Regexp *regexp.Regexp
}

// HTTPURLRegexp a slice of regexp to match URLs
type HTTPURLRegexp []urlRegexp

// String HTTPURLRegexp to string method
func (r *HTTPURLRegexp) String() string {
	return fmt.Sprint(*r)
}

// Set method to implement flags.Value
func (r *HTTPURLRegexp) Set(value string) error {
	regularExp, err := regexp.Compile(value)
	if err != nil {
		return err
	}

	*r = append(*r, urlRegexp{Regexp: regularExp})

	return nil
}
