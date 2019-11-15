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
// Connect to Consul. You can use it outside of Configify, too.
client, err := api.NewClient(api.DefaultConfig())
if err != nil {
	// Handle as you see fit...
}

// Create a source to read from Consul 
source, err = consul.NewSource(consul.Options{
    Client:  client,
    Options: configify.Options{Context: context.Background()},
})
if err != nil {
	// Handle as you see fit...
}

// Just like any configify.Source, you can read values individually...
stringValue, ok := source.String("HTTP_HOST")
int16Value, ok := source.Int16("HTTP_PORT")
...
```

## Struct Binding

You can look at the Struct Binding example in `configify` proper. All
you need to do is feed your Consul-backed source to the binder rather
than the Environment-backed binder.

https://github.com/robsignorelli/configify#struct-binding
