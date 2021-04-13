# Elasticsearch Hook for [Logrus](https://github.com/sirupsen/logrus) <img src="http://i.imgur.com/hTeVwmJ.png" width="40" height="40" alt=":walrus:" class="emoji" title=":walrus:"/>
<img src="https://travis-ci.org/bgsrb/elogrus.svg?branch=master" />

## Intro

This is a hard fork of https://github.com/sohlich/elogrus. The original library used https://github.com/olivere/elastic, which is a non-official Elasticsearch go client. This fork uses [the official client library](https://github.com/elastic/go-elasticsearch).

## Releases
This fork is designed to use the official Go client for Elasticsearch

**Notice that the master branch always refers to the latest version of Elastic. If you want to use stable versions of elogus, you should use the packages released via [gopkg.in](https://gopkg.in).**

*Here's the version matrix:*

Elasticsearch version | Elastic Go Client version  | Package URL                                                          | Remarks |
----------------------|----------------------------|----------------------------------------------------------------------|---------|
6.x                   | 6.0                        | [`github.com/bgsrb/elogrus/v6`](https://github.com/bgsrb/elogrus/v6)| Actively maintained.

*For Elasticsearch 6.x*
```bash
# We name v6 to align with elastic v6
go get github.com/elastic/go-elasticsearch/v6
go get github.com/bgsrb/elogrus/v6
```

## Changelog
- make a fork and switch to the official [The official Go client for Elasticsearch](https://github.com/elastic/go-elasticsearch).

## Usage

```go
package main

import (
	"github.com/elastic/go-elasticsearch/v6"
	"github.com/sirupsen/logrus"
	"github.com/bgsrb/elogrus/v6"
)

func main() {
	log := logrus.New()
	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{"http://127.0.0.1:7777"},
	})
	if err != nil {
		log.Panic(err)
	}
	hook, err := elogrus.NewBulkProcessorElasticHook(client, "localhost", logrus.DebugLevel, "mylog")
	if err != nil {
		log.Panic(err)
	}
	log.Hooks.Add(hook)
	log.WithFields(logrus.Fields{
		"name": "joe",
		"age":  42,
	}).Error("Hello world!")
}
```

### Asynchronous hook

```go
	...
	elogrus.NewAsyncElasticHook(client, "localhost", logrus.DebugLevel, "mylog")
	...
```
