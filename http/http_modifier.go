package http

import (
	"bytes"
	"encoding/base64"
	"hash/fnv"
	"strings"

	config "goreplay/config"

	"goreplay/proto"
)

// Modifier http Modifier
type Modifier struct {
	config *config.HTTPModifierConfig
}

// NewHTTPModifier new a http modifier
func NewHTTPModifier(conf *config.HTTPModifierConfig) *Modifier {
	// Optimization to skip modifier completely if we do not need it
	if len(conf.URLRegexp) == 0 &&
		len(conf.URLNegativeRegexp) == 0 &&
		len(conf.URLRewrite) == 0 &&
		len(conf.HeaderRewrite) == 0 &&
		checkFilters(conf) &&
		len(conf.Params) == 0 &&
		len(conf.Headers) == 0 &&
		len(conf.Methods) == 0 {
		return nil
	}

	return &Modifier{config: conf}
}

func (m *Modifier) isHeaderNegativeFiltersMatch(payload []byte) bool {
	if len(m.config.HeaderNegativeFilters) <= 0 {
		return false
	}

	for _, f := range m.config.HeaderNegativeFilters {
		value := proto.Header(payload, f.Name)

		if len(value) > 0 && f.Regexp.Match(value) {
			return true
		}
	}

	return false
}

// checkFilters check if the filters are nil
func checkFilters(conf *config.HTTPModifierConfig) bool {
	if len(conf.HeaderFilters) == 0 &&
		len(conf.HeaderNegativeFilters) == 0 &&
		len(conf.HeaderBasicAuthFilters) == 0 &&
		len(conf.HeaderHashFilters) == 0 &&
		len(conf.ParamHashFilters) == 0 {
		return true
	}

	return false
}

func (m *Modifier) isHeaderBasicAuthFiltersMatch(payload []byte) bool {
	if len(m.config.HeaderBasicAuthFilters) <= 0 {
		return false
	}

	for _, f := range m.config.HeaderBasicAuthFilters {
		value := proto.Header(payload, []byte("Authorization"))

		if len(value) > 0 {
			valueString := string(value)
			trimmedBasicAuthEncoded := strings.TrimPrefix(valueString, "Basic ")
			if strings.Compare(valueString, trimmedBasicAuthEncoded) != 0 {
				decodedAuth, _ := base64.StdEncoding.DecodeString(trimmedBasicAuthEncoded)
				if !f.Regexp.Match(decodedAuth) {
					return true
				}
			}
		}
	}

	return false
}

func (m *Modifier) matcherFilter(payload []byte) (response []byte) {
	switch {
	case m.isURLNegativeRegexp(payload):
		return
	case m.isHeaderFiltersMatch(payload):
		return
	case m.isHeaderNegativeFiltersMatch(payload):
		return
	case m.isHeaderBasicAuthFiltersMatch(payload):
		return
	case m.isHeaderHashFilters(payload):
		return
	case m.isParamHashFilters(payload):
		return
	default:
		return payload
	}
}

// Rewrite rewrite the http request
func (m *Modifier) Rewrite(payload []byte) (response []byte) {
	if !proto.HasRequestTitle(payload) {
		return payload
	}

	payload = m.dealHeaderAndParams(payload)
	payload = m.matcherFilter(payload)
	payload = m.dealRegExp(payload)

	if len(m.config.URLRewrite) > 0 {
		payload = m.dealURLRewrite(payload)
	}

	if len(m.config.HeaderRewrite) > 0 {
		payload = m.dealHeaderRewrite(payload)
	}

	return payload
}

func (m *Modifier) isURLRegexpMatch(payload []byte) bool {
	path := proto.Path(payload)
	matched := false

	for _, f := range m.config.URLRegexp {
		if f.Regexp.Match(path) {
			matched = true
			break
		}
	}
	return matched
}

func (m *Modifier) isParamHashFilters(payload []byte) bool {
	if len(m.config.ParamHashFilters) <= 0 {
		return false
	}

	for _, f := range m.config.ParamHashFilters {
		value, s, _ := proto.PathParam(payload, f.Name)

		if s != -1 {
			hasher := fnv.New32a()
			_, _ = hasher.Write(value)

			if (hasher.Sum32() % 100) >= f.Percent {
				return true
			}
		}
	}

	return false

}

func (m *Modifier) isHeaderHashFilters(payload []byte) bool {
	if len(m.config.HeaderHashFilters) <= 0 {
		return false
	}

	for _, f := range m.config.HeaderHashFilters {
		value := proto.Header(payload, f.Name)

		if len(value) > 0 {
			hasher := fnv.New32a()
			_, _ = hasher.Write(value)

			if (hasher.Sum32() % 100) >= f.Percent {
				return true
			}
		}
	}

	return false
}

func (m *Modifier) dealHeaderAndParams(payload []byte) []byte {
	if len(m.config.Headers) > 0 {
		for _, header := range m.config.Headers {
			payload = proto.SetHeader(payload, []byte(header.Name), []byte(header.Value))
		}
	}

	if len(m.config.Params) > 0 {
		for _, param := range m.config.Params {
			payload = proto.SetPathParam(payload, param.Name, param.Value)
		}
	}

	return payload
}

func (m *Modifier) isMethodsMatch(payload []byte) bool {
	method := proto.Method(payload)
	matched := false

	for _, m := range m.config.Methods {
		if bytes.Equal(method, m) {
			matched = true
			break
		}
	}

	return matched
}

func (m *Modifier) dealURLRewrite(payload []byte) []byte {
	path := proto.Path(payload)

	for _, f := range m.config.URLRewrite {
		if f.Regexp.Match(path) {
			path = f.Regexp.ReplaceAll(path, f.Target)
			payload = proto.SetPath(payload, path)

			break
		}
	}

	return payload
}

func (m *Modifier) dealHeaderRewrite(payload []byte) []byte {
	for _, f := range m.config.HeaderRewrite {
		value := proto.Header(payload, f.Header)
		if len(value) == 0 {
			break
		}

		if f.Regexp.Match(value) {
			newValue := f.Regexp.ReplaceAll(value, f.Target)
			payload = proto.SetHeader(payload, f.Header, newValue)
		}
	}
	return payload
}

func (m *Modifier) isURLNegativeRegexp(payload []byte) bool {
	if len(m.config.URLNegativeRegexp) <= 0 {
		return false
	}

	path := proto.Path(payload)

	for _, f := range m.config.URLNegativeRegexp {
		if f.Regexp.Match(path) {
			return true
		}
	}

	return false
}

func (m *Modifier) dealRegExp(payload []byte) (response []byte) {
	switch {
	case len(m.config.Methods) > 0 && !m.isMethodsMatch(payload):
		return
	case len(m.config.URLRegexp) > 0 && !m.isURLRegexpMatch(payload):
		return
	default:
		return payload
	}
}

func (m *Modifier) isHeaderFiltersMatch(payload []byte) bool {
	if len(m.config.HeaderFilters) <= 0 {
		return false
	}

	for _, f := range m.config.HeaderFilters {
		value := proto.Header(payload, f.Name)

		if len(value) == 0 {
			return true
		}

		if !f.Regexp.Match(value) {
			return true
		}
	}

	return false
}
