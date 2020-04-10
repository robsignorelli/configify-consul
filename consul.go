package consul

import (
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/pkg/errors"
	"github.com/robsignorelli/configify"
)

// NewSource creates a new config source that is backed by a Consul Key/Value store. You
// provide the consul client (so you can share connections w/ your service discovery and such)
// and this will extract config values for you.
func NewSource(opts ...configify.Option) (configify.SourceWatcher, error) {
	options := apply(opts, &configify.Options{
		Defaults:        configify.Empty(),
		RefreshInterval: 10 * time.Second,
	})

	if options.Context == nil {
		return nil, errors.New("consul source: missing context option")
	}
	if options.Address == "" {
		return nil, errors.New("consul source: missing address option")
	}

	client, err := api.NewClient(toConsulConfig(*options))
	if err != nil {
		return nil, errors.Wrapf(err, "consul source: connect error")
	}
	source := consulSource{
		client:  client,
		kv:      client.KV(),
		options: *options,
		massage: configify.Massage{},
	}

	// start w/ a full set of values and then listen() to have periodic refreshes.
	source.refresh()
	return &source, source.listen()
}

func toConsulConfig(options configify.Options) *api.Config {
	consulConfig := api.DefaultConfig()
	consulConfig.Address = options.Address
	if options.Username != "" || options.Password != "" {
		consulConfig.HttpAuth = &api.HttpBasicAuth{
			Username: options.Username,
			Password: options.Password,
		}
	}
	return consulConfig
}

func apply(options []configify.Option, defaults *configify.Options) *configify.Options {
	for _, option := range options {
		option(defaults)
	}
	return defaults
}

type consulSource struct {
	client    *api.Client
	kv        *api.KV
	options   configify.Options
	massage   configify.Massage
	values    map[string]string
	lastIndex uint64
	watcher   func(source configify.Source)
}

func (c consulSource) Options() configify.Options {
	return c.options
}

func (c *consulSource) listen() error {
	go func(source *consulSource) {
		for {
			select {
			case <-source.options.Context.Done():
				return
			case <-time.After(c.Options().RefreshInterval):
				break
			}
			// We do a refresh when we first set up the source, so don't fire off a second
			// refresh until the first timeout.
			source.refresh()
		}
	}(c)
	return nil
}

func (c *consulSource) refresh() {
	pairs, meta, err := c.kv.List(c.options.Namespace.Name, nil)
	if err != nil {
		return
	}
	// You already have the most up to date values
	if meta.LastIndex <= c.lastIndex {
		return
	}

	// Convert the slice of pairs to a quick-to-lookup map
	updatedValues := map[string]string{}
	for _, pair := range pairs {
		updatedValues[pair.Key] = string(pair.Value)
	}

	c.lastIndex = meta.LastIndex
	c.values = updatedValues

	// You can't set up a watcher until we've done the initial refresh() in
	// NewSource(), so this is guaranteed to only fire on subsequent auto-updates.
	if c.watcher != nil {
		c.watcher(c)
	}
}

func (c consulSource) lookup(key string) (string, bool) {
	if value, ok := c.values[c.options.Namespace.Qualify(key)]; ok {
		return strings.TrimSpace(value), true
	}
	return "", false
}

func (c *consulSource) Watch(callback func(source configify.Source)) {
	c.watcher = callback
}

func (c consulSource) String(key string) (string, bool) {
	value, ok := c.lookup(key)
	if !ok {
		return c.options.Defaults.String(key)
	}
	return value, true
}

func (c consulSource) StringSlice(key string) ([]string, bool) {
	value, ok := c.lookup(key)
	if !ok {
		return c.options.Defaults.StringSlice(key)
	}
	return c.massage.StringToSlice(value)
}

func (c consulSource) Int(key string) (int, bool) {
	value, ok := c.lookup(key)
	if !ok {
		return c.options.Defaults.Int(key)
	}
	number, ok := c.massage.StringToInt64(value)
	return int(number), ok
}

func (c consulSource) Int8(key string) (int8, bool) {
	value, ok := c.lookup(key)
	if !ok {
		return c.options.Defaults.Int8(key)
	}
	number, ok := c.massage.StringToInt64(value)
	return int8(number), ok
}

func (c consulSource) Int16(key string) (int16, bool) {
	value, ok := c.lookup(key)
	if !ok {
		return c.options.Defaults.Int16(key)
	}
	number, ok := c.massage.StringToInt64(value)
	return int16(number), ok
}

func (c consulSource) Int32(key string) (int32, bool) {
	value, ok := c.lookup(key)
	if !ok {
		return c.options.Defaults.Int32(key)
	}
	number, ok := c.massage.StringToInt64(value)
	return int32(number), ok
}

func (c consulSource) Int64(key string) (int64, bool) {
	value, ok := c.lookup(key)
	if !ok {
		return c.options.Defaults.Int64(key)
	}
	return c.massage.StringToInt64(value)
}

func (c consulSource) Uint(key string) (uint, bool) {
	value, ok := c.lookup(key)
	if !ok {
		return c.options.Defaults.Uint(key)
	}
	number, ok := c.massage.StringToUint64(value)
	return uint(number), ok
}

func (c consulSource) Uint8(key string) (uint8, bool) {
	value, ok := c.lookup(key)
	if !ok {
		return c.options.Defaults.Uint8(key)
	}
	number, ok := c.massage.StringToUint64(value)
	return uint8(number), ok
}

func (c consulSource) Uint16(key string) (uint16, bool) {
	value, ok := c.lookup(key)
	if !ok {
		return c.options.Defaults.Uint16(key)
	}
	number, ok := c.massage.StringToUint64(value)
	return uint16(number), ok
}

func (c consulSource) Uint32(key string) (uint32, bool) {
	value, ok := c.lookup(key)
	if !ok {
		return c.options.Defaults.Uint32(key)
	}
	number, ok := c.massage.StringToUint64(value)
	return uint32(number), ok
}

func (c consulSource) Uint64(key string) (uint64, bool) {
	value, ok := c.lookup(key)
	if !ok {
		return c.options.Defaults.Uint64(key)
	}
	return c.massage.StringToUint64(value)
}

func (c consulSource) Float32(key string) (float32, bool) {
	value, ok := c.lookup(key)
	if !ok {
		return c.options.Defaults.Float32(key)
	}
	number, ok := c.massage.StringToFloat64(value)
	return float32(number), ok
}

func (c consulSource) Float64(key string) (float64, bool) {
	value, ok := c.lookup(key)
	if !ok {
		return c.options.Defaults.Float64(key)
	}
	return c.massage.StringToFloat64(value)
}

func (c consulSource) Bool(key string) (bool, bool) {
	value, ok := c.lookup(key)
	if !ok {
		return c.options.Defaults.Bool(key)
	}
	return c.massage.StringToBool(value)
}

func (c consulSource) Duration(key string) (time.Duration, bool) {
	value, ok := c.lookup(key)
	if !ok {
		return c.options.Defaults.Duration(key)
	}
	return c.massage.StringToDuration(value)
}

func (c consulSource) Time(key string) (time.Time, bool) {
	value, ok := c.lookup(key)
	if !ok {
		return c.options.Defaults.Time(key)
	}
	return c.massage.StringToTime(value)
}
