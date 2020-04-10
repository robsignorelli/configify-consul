[![Go Report Card](https://goreportcard.com/badge/github.com/robsignorelli/configify-consul)](https://goreportcard.com/report/github.com/robsignorelli/configify-consul)

# Configify: Consul Plugin

This is the Consul plugin for [Configify](https://github.com/robsignorelli/configify),
a Go library for grabbing values from various places that have your
program's configuration data. You can use data from your Consul
cluster's Key/Value exactly the same way you would use your OS's
environment variables.

## Getting Started

```
go get github.com/robsignorelli/configify-consul
```

## Basic Usage

You'll need to provide your own Consul client instance. This way
if you're also using Consul for service discovery, you can reuse
that connection to read from your Key/Values as well.

```go
source, err := consul.NewSource(
	configify.Context(suite.context),
	configify.Address("consul.host:8500"),
	configify.Namespace("FOO"),
	configify.NamespaceDelim("/"),
	configify.RefreshInterval(5*time.Second))

if err != nil {
	// Handle as you see fit...
}

// Just like any configify.Source, you can read values individually. It
// will also honor namespace prefixes so you're really looking up the
// values "FOO/HTTP_HOST" and "FOO/HTTP_PORT".
stringValue, ok := source.String("HTTP_HOST")
int16Value, ok := source.Int16("HTTP_PORT")
...
```

## Struct Binding

You can look at the Struct Binding example in `configify` proper. All
you need to do is feed your Consul-backed source to the binder rather
than the Environment-backed binder.

https://github.com/robsignorelli/configify#struct-binding

## Watch for Updates

You can let configify automatically fire a callback whenever we detect
changes to your app's config, so you can react accordingly. Now you
can update your configuration w/o having to restart your program.

```go
source, _ := consul.NewSource(...)
source.Watch(func (updated configify.Source) {
	myConfig.RetryCount, _ := updated.Int("RETRY_COUNT")
	myConfig.SomeToken, _ := updated.String("SOME_TOKEN")
	...
})
```

Be aware that your `Watcher` callback does NOT fire when the Consul source
loads config values for the first time (i.e. when you call `NewSource`).
It only fires when it detects a modification to the KV store any time
after the source was initialized.
