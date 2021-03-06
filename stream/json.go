package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gopub/gox"
	"github.com/gopub/log"
	"github.com/gopub/wine"
	"github.com/gopub/wine/api"
	"github.com/gopub/wine/mime"
)

type JSONReadCloser interface {
	Read(v interface{}) error
	io.Closer
}

type JSONWriteCloser interface {
	Write(v interface{}) error
	io.Closer
}

type jsonReadCloser struct {
	textReadCloser
}

func newJSONReadCloser(body io.ReadCloser) *jsonReadCloser {
	r := newTextReadCloser(body)
	return &jsonReadCloser{textReadCloser: *r}
}

func (r *jsonReadCloser) Read(v interface{}) error {
	p, err := r.textReadCloser.Read()
	if err != nil {
		return fmt.Errorf("read text: %w", err)
	}
	err = json.Unmarshal([]byte(p), v)
	if err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	return nil
}

type jsonWriteCloser struct {
	textWriteCloser
}

func newJSONWriteCloser(w http.ResponseWriter, done chan<- interface{}) *jsonWriteCloser {
	r := newTextWriteCloser(w, done)
	return &jsonWriteCloser{textWriteCloser: *r}
}

func (w *jsonWriteCloser) Write(v interface{}) error {
	p, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	err = w.textWriteCloser.Write(string(p))
	if err != nil {
		return fmt.Errorf("write text: %w", err)
	}
	return nil
}

func NewJSONReader(client *http.Client, req *http.Request) (JSONReadCloser, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		err = api.ParseResult(resp, nil, true)
		if err != nil {
			return nil, fmt.Errorf("parse result: %w", err)
		}
		return nil, gox.NewError(resp.StatusCode, "unknown error")
	}
	return newJSONReadCloser(resp.Body), nil
}

func NewJSONHandler(serve func(context.Context, JSONWriteCloser)) wine.Handler {
	return wine.HandlerFunc(func(ctx context.Context, req *wine.Request, next wine.Invoker) wine.Responder {
		logger := log.FromContext(ctx)
		logger.Debugf("Receive stream")
		w := wine.GetResponseWriter(ctx)
		w.Header().Set(mime.ContentType, mime.JSON)
		done := make(chan interface{})
		go serve(ctx, newJSONWriteCloser(w, done))
		<-done
		logger.Debugf("Close stream")
		return wine.Status(http.StatusOK)
	})
}
