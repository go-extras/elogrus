package elogrus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/elastic/go-elasticsearch/v6"
	"github.com/elastic/go-elasticsearch/v6/esapi"
	"github.com/sirupsen/logrus"

	"gopkg.in/go-extras/elogrus.v6/internal/bulk"
)

var (
	// ErrCannotCreateIndex Fired if the index is not created
	ErrCannotCreateIndex = fmt.Errorf("cannot create index")
)

// IndexNameFunc get index name
type IndexNameFunc func() string

type fireFunc func(entry *logrus.Entry, hook *ElasticHook) error

// ElasticHook is a logrus
// hook for ElasticSearch
type ElasticHook struct {
	client    *elasticsearch.Client
	host      string
	index     IndexNameFunc
	levels    []logrus.Level
	ctx       context.Context
	ctxCancel context.CancelFunc
	fireFunc  fireFunc
}

type message struct {
	Host      string        `json:"host"`
	Timestamp string        `json:"@timestamp"`
	Message   string        `json:"message"`
	Data      logrus.Fields `json:"data"`
	Level     string        `json:"level"`
}

// NewElasticHook creates new hook.
// client - ElasticSearch client with specific es version (v5/v6/v7/...)
// host - host of system
// level - log level
// index - name of the index in ElasticSearch
func NewElasticHook(client *elasticsearch.Client, host string, level logrus.Level, index string) (*ElasticHook, error) {
	return NewElasticHookWithFunc(client, host, level, func() string { return index })
}

// NewAsyncElasticHook creates new  hook with asynchronous log.
// client - ElasticSearch client with specific es version (v5/v6/v7/...)
// host - host of system
// level - log level
// index - name of the index in ElasticSearch
func NewAsyncElasticHook(client *elasticsearch.Client, host string, level logrus.Level, index string) (*ElasticHook, error) {
	return NewAsyncElasticHookWithFunc(client, host, level, func() string { return index })
}

// NewBulkProcessorElasticHook creates new hook that uses a bulk processor for indexing.
// client - ElasticSearch client with specific es version (v5/v6/v7/...)
// host - host of system
// level - log level
// index - name of the index in ElasticSearch
func NewBulkProcessorElasticHook(client *elasticsearch.Client, host string, level logrus.Level, index string) (*ElasticHook, error) {
	return NewBulkProcessorElasticHookWithFunc(client, host, level, func() string { return index })
}

// NewElasticHookWithFunc creates new hook with
// function that provides the index name. This is useful if the index name is
// somehow dynamic especially based on time.
// client - ElasticSearch client with specific es version (v5/v6/v7/...)
// host - host of system
// level - log level
// indexFunc - function providing the name of index
func NewElasticHookWithFunc(client *elasticsearch.Client, host string, level logrus.Level, indexFunc IndexNameFunc) (*ElasticHook, error) {
	return newHookFuncAndFireFunc(client, host, level, indexFunc, syncFireFunc)
}

// NewAsyncElasticHookWithFunc creates new asynchronous hook with
// function that provides the index name. This is useful if the index name is
// somehow dynamic especially based on time.
// client - ElasticSearch client with specific es version (v5/v6/v7/...)
// host - host of system
// level - log level
// indexFunc - function providing the name of index
func NewAsyncElasticHookWithFunc(client *elasticsearch.Client, host string, level logrus.Level, indexFunc IndexNameFunc) (*ElasticHook, error) {
	return newHookFuncAndFireFunc(client, host, level, indexFunc, asyncFireFunc)
}

// NewBulkProcessorElasticHookWithFunc creates new hook with
// function that provides the index name. This is useful if the index name is
// somehow dynamic especially based on time that uses a bulk processor for
// indexing.
// client - ElasticSearch client with specific es version (v5/v6/v7/...)
// host - host of system
// level - log level
// indexFunc - function providing the name of index
func NewBulkProcessorElasticHookWithFunc(client *elasticsearch.Client, host string, level logrus.Level, indexFunc IndexNameFunc) (*ElasticHook, error) {
	fireFunc, err := makeBulkFireFunc(client)
	if err != nil {
		return nil, err
	}
	return newHookFuncAndFireFunc(client, host, level, indexFunc, fireFunc)
}

func newHookFuncAndFireFunc(client *elasticsearch.Client, host string, level logrus.Level, indexFunc IndexNameFunc, fireFunc fireFunc) (*ElasticHook, error) {
	var levels []logrus.Level
	for _, l := range []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
		logrus.TraceLevel,
	} {
		if l <= level {
			levels = append(levels, l)
		}
	}

	ctx, cancel := context.WithCancel(context.TODO())

	// Use the IndexExists service to check if a specified index exists.
	indexExistsResp, err := client.Indices.Exists([]string{indexFunc()})
	if err != nil {
		// Handle error
		cancel()
		return nil, err
	}
	if indexExistsResp.StatusCode == http.StatusNotFound {
		createIndexResp, err := client.Indices.Create(indexFunc())
		if err != nil || createIndexResp.IsError() {
			cancel()
			return nil, ErrCannotCreateIndex
		}
	}

	return &ElasticHook{
		client:    client,
		host:      host,
		index:     indexFunc,
		levels:    levels,
		ctx:       ctx,
		ctxCancel: cancel,
		fireFunc:  fireFunc,
	}, nil
}

// Fire is required to implement
// Logrus hook
func (hook *ElasticHook) Fire(entry *logrus.Entry) error {
	return hook.fireFunc(entry, hook)
}

func asyncFireFunc(entry *logrus.Entry, hook *ElasticHook) error {
	go func() {
		_ = syncFireFunc(entry, hook) // TODO: how can we handle the error?
	}()
	return nil
}

func createMessage(entry *logrus.Entry, hook *ElasticHook) *message {
	level := entry.Level.String()

	if e, ok := entry.Data[logrus.ErrorKey]; ok && e != nil {
		if err, ok := e.(error); ok {
			entry.Data[logrus.ErrorKey] = err.Error()
		}
	}

	return &message{
		hook.host,
		entry.Time.UTC().Format(time.RFC3339Nano),
		entry.Message,
		entry.Data,
		strings.ToUpper(level),
	}
}

func syncFireFunc(entry *logrus.Entry, hook *ElasticHook) error {
	data, err := json.Marshal(createMessage(entry, hook))
	if err != nil {
		return err
	}
	req := esapi.IndexRequest{
		Index: hook.index(),
		Body:  bytes.NewReader(data),
	}

	// Perform the request with the client.
	res, err := req.Do(context.Background(), hook.client)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	return err
}

// Create closure with bulk processor tied to fireFunc.
// Note: garbage collector will never be able to free memory allocated for bulkWriters
func makeBulkFireFunc(client *elasticsearch.Client) (fireFunc, error) {
	bulkWriters := make(map[*ElasticHook]*bulk.Writer)
	var lock sync.RWMutex

	getWriter := func(hook *ElasticHook) *bulk.Writer {
		lock.RLock()
		writer := bulkWriters[hook]
		lock.RUnlock()
		if writer != nil {
			return writer
		}
		lock.Lock()
		writer = bulkWriters[hook]
		if writer != nil { // this is a second check to avoid sequential writes
			lock.Unlock()
			return writer
		}

		// long path, create a new writer
		writer = bulk.NewBulkWriterWithErrorHandler(time.Second, func(data []byte) error {
			res, err := client.Bulk(bytes.NewReader(data),
				client.Bulk.WithIndex(hook.index()),
			)
			if err != nil {
				return err
			}
			defer res.Body.Close()
			if res.IsError() {
				raw := make(map[string]interface{})
				if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
					return fmt.Errorf("failure to to parse response body: %s", err.Error())
				} else {
					return fmt.Errorf("error: [%d] %s: %s",
						res.StatusCode,
						raw["error"].(map[string]interface{})["type"],
						raw["error"].(map[string]interface{})["reason"],
					)
				}
				// A successful response might still contain errors for particular documents...
				//
			}
			return nil
		}, func(data []byte, err error) {
			// TODO: how to handle the error??
			// panic(fmt.Sprintf("error: %s", err))
		})
		bulkWriters[hook] = writer
		lock.Unlock()

		return writer
	}

	return func(entry *logrus.Entry, hook *ElasticHook) error {
		data, err := json.Marshal(createMessage(entry, hook))
		if err != nil {
			return err
		}
		data = append([]byte(`{"index":{}}`+"\n"), data...)
		_, _ = getWriter(hook).Write(append(data, '\n'))
		return nil
	}, nil
}

// Levels Required for logrus hook implementation
func (hook *ElasticHook) Levels() []logrus.Level {
	return hook.levels
}

// Cancel all calls to elastic
func (hook *ElasticHook) Cancel() {
	hook.ctxCancel()
}
