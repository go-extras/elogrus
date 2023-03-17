# Elasticsearch Hook for [Logrus](https://github.com/sirupsen/logrus) <img src="http://i.imgur.com/hTeVwmJ.png" width="40" height="40" alt=":walrus:" class="emoji" title=":walrus:"/>
<img src="https://travis-ci.org/go-extras/elogrus.svg?branch=master" />

## Intro

This is a hard fork of https://github.com/sohlich/elogrus. The original library used https://github.com/olivere/elastic, which is a non-official Elasticsearch go client. This fork uses [the official client library](https://github.com/elastic/go-elasticsearch).

## Releases
This fork is designed to use the official Go client for Elasticsearch

**Notice that the master branch always refers to the latest version of Elastic. If you want to use stable versions of elogus, you should use the packages released via [gopkg.in](https://gopkg.in).**

*Here's the version matrix:*

Elasticsearch version | Elastic Go Client version | Package URL                                                              | Remarks |
----------------------|---------------------------|--------------------------------------------------------------------------|---------|
7.x                   | 7.0                       | [`gopkg.in/go-extras/elogrus.v7`](https://gopkg.in/go-extras/elogrus.v7) | Actively maintained.
8.x                   | 8.0                       | [`gopkg.in/go-extras/elogrus.v8`](https://gopkg.in/go-extras/elogrus.v8) | Actively maintained.

*For Elasticsearch 7.x*
```bash
# We name v7 to align with elastic v7
go get github.com/elastic/go-elasticsearch/v7
go get gopkg.in/go-extras/elogrus.v7
```

*For Elasticsearch 8.x*
```bash
# We name v8 to align with elastic v8
go get github.com/elastic/go-elasticsearch/v8
go get gopkg.in/go-extras/elogrus.v8
```

## Usage

```go
package main

import (
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/sirupsen/logrus"
	"gopkg.in/go-extras/elogrus.v8"
)

func main() {
	log := logrus.New()
	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{"http://127.0.0.1:7777"},
	})
	if err != nil {
		log.Panic(err)
	}
	hook, err := elogrus.NewAsyncElasticHook(client, "localhost", logrus.DebugLevel, "mylog")
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

### ECS Logging

It is possible to produce log entries compatible with [ECS Logging format](https://www.elastic.co/guide/en/ecs-logging/overview/current/intro.html) using
the [official ECS library](https://www.elastic.co/guide/en/ecs-logging/go-logrus/current/intro.html) for `logrus`.

```go
package main

import (
	"json"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/sirupsen/logrus"
	"go.elastic.co/ecslogrus"
	"gopkg.in/go-extras/elogrus.v8"
)

func ECSLogMessageModifierFunc(formatter *ecslogrus.Formatter) func(*logrus.Entry, *elogrus.Message) any {
	return func(entry *logrus.Entry, message *elogrus.Message) any {
		var data json.RawMessage
		data, err := formatter.Format(entry)
		if err != nil {
			return entry // in case of an error just preserve the original entry
		}
		return data
	}

}

func main() {
	// ...
	hook.MessageModifierFunc = ECSLogMessageModifierFunc(&ecslogrus.Formatter{})
	// ...
}
```