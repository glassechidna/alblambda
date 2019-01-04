package alblambda

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
)

type Wrapper struct {
	MultiValuedHeaders bool

	handler http.Handler
}

func Wrap(handler http.Handler) *Wrapper {
	return &Wrapper{handler: handler}
}

func (w *Wrapper) LambdaHandle(ctx context.Context, request events.ALBTargetGroupRequest) (events.ALBTargetGroupResponse, error) {
	writer := httptest.NewRecorder()
	r := makeHttpRequest(request, w.MultiValuedHeaders)
	w.handler.ServeHTTP(writer, r)
	resp := writer.Result()
	return makeHttpResponse(resp, w.MultiValuedHeaders), nil
}

func makeHttpResponse(resp *http.Response, multiValued bool) events.ALBTargetGroupResponse {
	body, _ := ioutil.ReadAll(resp.Body)
	b64 := base64.RawStdEncoding.EncodeToString(body)

	ret := events.ALBTargetGroupResponse{
		Body:              b64,
		IsBase64Encoded:   true,
		StatusCode:        resp.StatusCode,
		StatusDescription: resp.Status,
	}

	for name, values := range resp.Header {
		if multiValued {
			ret.MultiValueHeaders[name] = values
		} else {
			ret.Headers[name] = values[0]
		}
	}

	return ret
}

func makeHttpRequest(inner events.ALBTargetGroupRequest, multiValued bool) *http.Request {
	u := urlForRequest(inner)

	var body io.Reader = bytes.NewReader([]byte(inner.Body))
	if inner.IsBase64Encoded {
		body = base64.NewDecoder(base64.RawStdEncoding, body)
	}

	req, _ := http.NewRequest(inner.HTTPMethod, u.String(), body)

	if multiValued {
		for k, vs := range inner.MultiValueHeaders {
			req.Header[k] = vs
		}
	} else {
		for k, v := range inner.Headers {
			req.Header.Set(k, v)
		}
	}

	return req
}

func urlForRequest(request events.ALBTargetGroupRequest) *url.URL {
	proto := request.Headers["x-forwarded-proto"]
	host := request.Headers["host"]
	path := request.Path

	query := url.Values{}
	for k, vs := range request.MultiValueQueryStringParameters {
		query[k] = vs
	}
	for k, v := range request.QueryStringParameters {
		query[k] = append(query[k], v)
	}

	u, _ := url.Parse(fmt.Sprintf("%s://%s%s?%s", proto, host, path, query.Encode()))
	return u
}
