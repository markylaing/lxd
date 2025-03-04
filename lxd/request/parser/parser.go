package parser

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

type PathArgumentGetter func(r *http.Request, key string) (string, error)
type PathArgumentMapGetter func(r *http.Request) map[string]string

type PathArgumentPostProcessor func(string) (string, error)

// New returns a Parser.
func New() *Parser {
	return &Parser{}
}

// Parser can be used a http.Request according to the given configuration. This is to reduce boilerplate in API
// routes and standardise error responses.
type Parser struct {
	pathArgumentGetter               PathArgumentGetter
	pathArgumentMapGetter            PathArgumentMapGetter
	pathArgumentFromMapPostProcessor PathArgumentPostProcessor
	requiredPathArgument             map[string]any
	optionalPathArgument             map[string]any
	requiredQueryParameters          map[string]any
	optionalQueryParameters          map[string]any
	requiredHeaders                  map[string]any
	optionalHeaders                  map[string]any
	body                             any
}

func (r *Parser) WithPathArgumentGetter(getter PathArgumentGetter) *Parser {
	r.pathArgumentGetter = getter
	return r
}

func (r *Parser) WithPathArgumentMapGetter(getter PathArgumentMapGetter, postProcessor PathArgumentPostProcessor) *Parser {
	r.pathArgumentMapGetter = getter
	r.pathArgumentFromMapPostProcessor = postProcessor
	return r
}

// WithBody sets the target for the http.Request.WithBody to be unmarshalled into.
func (r *Parser) WithBody(target any) *Parser {
	r.body = target

	return r
}

// WithRequiredPathArgument configures the Parser to read the given key from path arguments into the target and error if it
// is not present.
func (r *Parser) WithRequiredPathArgument(key string, target any) *Parser {
	if r.requiredPathArgument == nil {
		r.requiredPathArgument = make(map[string]any)
	}

	r.requiredPathArgument[key] = target

	return r
}

// WithOptionalPathArgument configures the Parser to read the given key from path arguments into the target. It will set an
// empty string if the variable is not present.
func (r *Parser) WithOptionalPathArgument(key string, target any) *Parser {
	if r.optionalPathArgument == nil {
		r.optionalPathArgument = make(map[string]any)
	}

	r.optionalPathArgument[key] = target

	return r
}

// WithRequiredQueryParameter configures the Parser to read the given key from the request query parameters into the
// target and error if it is not present.
func (r *Parser) WithRequiredQueryParameter(key string, target any) *Parser {
	if r.requiredQueryParameters == nil {
		r.requiredQueryParameters = make(map[string]any)
	}

	r.requiredQueryParameters[key] = target

	return r
}

// WithOptionalQueryParameter configures the Parser to read the given key from the request query parameters into the
// target. It will set an empty string if the variable is not present.
func (r *Parser) WithOptionalQueryParameter(key string, target any) *Parser {
	if r.optionalQueryParameters == nil {
		r.optionalQueryParameters = make(map[string]any)
	}

	r.optionalQueryParameters[key] = target

	return r
}

// WithRequiredHeader configures the Parser to read the given key from the request headers into the
// target and error if it is not present.
func (r *Parser) WithRequiredHeader(key string, target any) *Parser {
	if r.requiredHeaders == nil {
		r.requiredHeaders = make(map[string]any)
	}

	r.requiredHeaders[key] = target

	return r
}

// WithOptionalHeader configures the Parser to read the given key from the request headers into the
// target. It will set an empty string if the variable is not present.
func (r *Parser) WithOptionalHeader(key string, target any) *Parser {
	if r.optionalHeaders == nil {
		r.optionalHeaders = make(map[string]any)
	}

	r.optionalHeaders[key] = target

	return r
}

// Parse reads all configured keys and/or the body into their targets.
func (r *Parser) Parse(request *http.Request) error {
	if len(r.requiredQueryParameters) > 0 {
		err := parseQueryString(request.URL, r.requiredQueryParameters, false)
		if err != nil {
			return err
		}
	}

	if len(r.optionalQueryParameters) > 0 {
		err := parseQueryString(request.URL, r.optionalQueryParameters, true)
		if err != nil {
			return err
		}
	}

	if len(r.requiredPathArgument) > 0 {
		if r.pathArgumentMapGetter == nil && r.pathArgumentGetter == nil {
			return errors.New("Path argument getter required")
		}

		err := r.parsePathArguments(request, r.requiredPathArgument, false)
		if err != nil {
			return err
		}
	}

	if len(r.optionalPathArgument) > 0 {
		if r.pathArgumentMapGetter == nil && r.pathArgumentGetter == nil {
			return errors.New("Path argument getter required")
		}

		err := r.parsePathArguments(request, r.optionalPathArgument, true)
		if err != nil {
			return err
		}
	}

	if len(r.requiredHeaders) > 0 {
		err := parseHeaders(request.Header, r.requiredHeaders, false)
		if err != nil {
			return err
		}
	}

	if len(r.optionalHeaders) > 0 {
		err := parseHeaders(request.Header, r.optionalHeaders, true)
		if err != nil {
			return err
		}
	}

	if r.body != nil {
		buf := bytes.NewBuffer(nil)
		teeReader := io.TeeReader(request.Body, buf)
		err := json.NewDecoder(teeReader).Decode(r.body)
		if err != nil {
			return api.StatusErrorf(http.StatusBadRequest, "Failed to parse request body: %v", err)
		}

		request.Body = io.NopCloser(buf)
	}

	return nil
}

// parsePathArguments reads required and optional path arguments keys and sets their values. Required keys must be non-empty.
func (r *Parser) parsePathArguments(req *http.Request, vars map[string]any, optional bool) error {
	var argMap map[string]string
	if r.pathArgumentMapGetter != nil {
		argMap = r.pathArgumentMapGetter(req)
	}

	for pathKey, target := range vars {
		var pathValue string
		if argMap != nil {
			pathValue = argMap[pathKey]
			if r.pathArgumentFromMapPostProcessor != nil {
				var err error
				pathValue, err = r.pathArgumentFromMapPostProcessor(pathValue)
				if err != nil {
					return err
				}
			}
		} else {
			var err error
			pathValue, err = r.pathArgumentGetter(req, pathKey)
			if err != nil {
				return err
			}
		}

		if !optional && pathValue == "" {
			return api.StatusErrorf(http.StatusBadRequest, "Missing required path parameter %q", pathKey)
		}

		if pathValue != "" {
			err := parseString(pathValue, target)
			if err != nil {
				return api.StatusErrorf(http.StatusBadRequest, "Failed to parse value of path parameter %q: %v", pathKey, err)
			}
		}
	}

	return nil
}

// parsePathArguments reads required and optional query parameters and sets their values. Required keys must be non-empty.
func parseQueryString(u *url.URL, keys map[string]any, optional bool) error {
	values, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return api.StatusErrorf(http.StatusBadRequest, "Malformed URL: %v", err)
	}

	for queryKey, target := range keys {
		queryValue := values.Get(queryKey)
		if !optional && queryValue == "" {
			return api.StatusErrorf(http.StatusBadRequest, "Missing required query parameter %q", queryKey)
		}

		if queryValue != "" {
			err := parseString(queryValue, target)
			if err != nil {
				return api.StatusErrorf(http.StatusBadRequest, "Failed to parse value of query parameter %q: %v", queryKey, err)
			}
		}
	}

	return nil
}

func parseHeaders(header http.Header, keys map[string]any, optional bool) error {
	for headerKey, target := range keys {
		headerValue := header.Get(headerKey)
		if !optional && headerValue == "" {
			return api.StatusErrorf(http.StatusBadRequest, "Missing required header %q", headerKey)
		}

		if headerValue != "" {
			err := parseString(headerValue, target)
			if err != nil {
				return api.StatusErrorf(http.StatusBadRequest, "Failed to parse header %q: %v", headerKey, err)
			}
		}
	}

	return nil
}

type StringSetter interface {
	SetString(string) error
}

// parseString parses the given string according to the type of the given target. If the string is parsed successfully,
// the value of the target is set to the parsed value.
func parseString(str string, target any) error {
	if target == nil {
		return api.StatusErrorf(http.StatusInternalServerError, "Cannot parse %q: Target is nil", str)
	}

	switch t := target.(type) {
	case StringSetter:
		return t.SetString(str)
	case *string:
		*t = str
	case *int:
		i, err := strconv.Atoi(str)
		if err != nil {
			return api.StatusErrorf(http.StatusBadRequest, "Could not parse %q into an integer: %w", str, err)
		}

		*t = i
	case *bool:
		*t = shared.IsTrue(str)
	default:
		return api.StatusErrorf(http.StatusInternalServerError, "Could not parse %q: Invalid target type", str)
	}

	return nil
}
