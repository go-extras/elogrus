package elogrus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v7"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"log"
	"net/http"
	"testing"
	"time"
)

type NewHookFunc func(client *elasticsearch.Client, host string, level logrus.Level, index string) (*ElasticHook, error)

func TestSyncHook(t *testing.T) {
	hookTest(NewElasticHook, "sync-log", t)
}

func TestAsyncHook(t *testing.T) {
	hookTest(NewAsyncElasticHook, "async-log", t)
}

func TestBulkProcessorHook(t *testing.T) {
	hookTest(NewBulkProcessorElasticHook, "bulk-log", t)
}

func hookTest(hookfunc NewHookFunc, indexName string, t *testing.T) {
	if r, err := http.Get("http://127.0.0.1:7777"); err != nil {
		log.Fatal("Elastic not reachable")
	} else {
		buf, _ := ioutil.ReadAll(r.Body)
		r.Body.Close()
		fmt.Println(string(buf))
	}

	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{"http://127.0.0.1:7777"},
	})

	if err != nil {
		log.Panic(err)
	}

	client.Indices.Delete([]string{indexName})

	hook, err := hookfunc(client, "localhost", logrus.DebugLevel, indexName)
	if err != nil {
		log.Panic(err)
	}
	logrus.AddHook(hook)

	samples := 100
	for index := 0; index < samples; index++ {
		logrus.Infof("Hustej msg %d", time.Now().Unix())
	}

	// Allow time for data to be processed.
	time.Sleep(2 * time.Second)

	var buf bytes.Buffer
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"host": map[string]interface{}{
					"value": "localhost",
				},
			},
		},
	}
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		log.Fatalf("Error encoding query: %s", err)
	}

	// Perform the search request.
	res, err := client.Search(
		client.Search.WithContext(context.Background()),
		client.Search.WithIndex(indexName),
		client.Search.WithBody(&buf),
		client.Search.WithTrackTotalHits(true),
		client.Search.WithPretty(),
	)
	if err != nil {
		t.Errorf("Search error: %v", err)
		t.FailNow()
	}
	defer res.Body.Close()

	if res.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			t.Errorf("Error parsing the response body: %v", err)
			t.FailNow()
		} else {
			// Print the response status and error information.
			t.Errorf("[%s] %s: %s",
				res.Status(),
				e["error"].(map[string]interface{})["type"],
				e["error"].(map[string]interface{})["reason"],
			)
			t.FailNow()
		}
	}

	var r map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		t.Fatalf("Error parsing the response body: %s", err)
	}

	if r["hits"] == nil {
		t.Fatalf("Missing hits")
	}
	if r["hits"].(map[string]interface{})["total"] == nil {
		t.Fatalf("Missing total")
	}
	if r["hits"].(map[string]interface{})["total"].(map[string]interface{})["value"] == nil {
		t.Fatalf("Missing value")
	}
	val, ok := r["hits"].(map[string]interface{})["total"].(map[string]interface{})["value"].(float64)
	if !ok {
		t.Fatalf("Value is not float64 %T", r["hits"].(map[string]interface{})["total"].(map[string]interface{})["value"])
	}
	if int(val) != samples {
		t.Errorf("Not all logs pushed to elastic: expected %d got %v", samples, val)
		t.FailNow()
	}
}

func TestError(t *testing.T) {
	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{"http://127.0.0.1:7777"},
	})

	if err != nil {
		log.Panic(err)
	}

	client.Indices.Delete([]string{"errorlog"})

	hook, err := NewElasticHook(client, "localhost", logrus.DebugLevel, "errorlog")
	if err != nil {
		log.Panic(err)
		t.FailNow()
	}
	logrus.AddHook(hook)

	logrus.WithError(fmt.Errorf("this is error")).
		Error("Failed to handle invalid api response")

	// Allow time for data to be processed.
	time.Sleep(1 * time.Second)

	var buf bytes.Buffer
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"host": map[string]interface{}{
					"value": "localhost",
				},
			},
		},
	}
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		log.Fatalf("Error encoding query: %s", err)
	}

	res, err := client.Search(
		client.Search.WithContext(context.Background()),
		client.Search.WithIndex("errorlog"),
		client.Search.WithBody(&buf),
		client.Search.WithTrackTotalHits(true),
		client.Search.WithPretty(),
	)
	if err != nil {
		t.Errorf("Search error: %v", err)
		t.FailNow()
	}
	defer res.Body.Close()

	if err != nil {
		t.Errorf("Search error: %v", err)
		t.FailNow()
	}
	defer res.Body.Close()

	if res.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			t.Errorf("Error parsing the response body: %v", err)
			t.FailNow()
		} else {
			// Print the response status and error information.
			t.Errorf("[%s] %s: %s",
				res.Status(),
				e["error"].(map[string]interface{})["type"],
				e["error"].(map[string]interface{})["reason"],
			)
			t.FailNow()
		}
	}

	var r map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		t.Fatalf("Error parsing the response body: %s", err)
	}

	if r["hits"] == nil {
		t.Fatalf("Missing hits")
	}
	if r["hits"].(map[string]interface{})["total"] == nil {
		t.Fatalf("Missing total")
	}
	if r["hits"].(map[string]interface{})["total"].(map[string]interface{})["value"] == nil {
		t.Fatalf("Missing value")
	}
	val, ok := r["hits"].(map[string]interface{})["total"].(map[string]interface{})["value"].(float64)
	if !ok {
		t.Fatalf("Value is not float64 %T", r["hits"].(map[string]interface{})["total"].(map[string]interface{})["value"])
	}
	if !(val > 1) {
		t.Error("No log created")
		t.FailNow()
	}

	if r["hits"].(map[string]interface{})["hits"] == nil {
		t.Fatalf("Missing hits.hits")
	}
	data, ok := r["hits"].(map[string]interface{})["hits"].([]map[string]interface{})
	if !ok {
		t.Fatalf("hits.hits is not []map[string]interface{}")
	}

	for _, v := range data {
		log.Println(v["_id"]) // TODO: write actual tests
	}
}
