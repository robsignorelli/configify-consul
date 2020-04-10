package consul_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/robsignorelli/configify"
	"github.com/robsignorelli/configify-consul"
	"github.com/robsignorelli/configify/configifytest"
	"github.com/stretchr/testify/suite"
)

var consulTestEndpoint = "127.0.0.1:8500"

func TestConsulSuite(t *testing.T) {
	suite.Run(t, new(ConsulSuite))
}

type ConsulSuite struct {
	configifytest.SourceSuite
	client        *api.Client
	kv            *api.KV
	context       context.Context
	contextCancel func()
}

func (suite *ConsulSuite) SetupTest() {
	var err error
	suite.client, err = api.NewClient(api.DefaultConfig())
	suite.Require().NoError(err, "unable to connect to consul backend")

	suite.kv = suite.client.KV()
	suite.resetStore()

	suite.context, suite.contextCancel = context.WithCancel(context.Background())
	suite.Source, err = consul.NewSource(
		configify.Context(suite.context),
		configify.Address(consulTestEndpoint),
		configify.Namespace("FOO"),
		configify.NamespaceDelim("/"),
		configify.RefreshInterval(50*time.Millisecond),
	)
	suite.Require().NoError(err, "unable to create consul source")
}

func (suite *ConsulSuite) TearDownTest() {
	if suite.contextCancel != nil {
		suite.contextCancel()
	}
}

func (suite *ConsulSuite) resetStore() {
	// Start our key/value bundle from a completely blank slate (yay, idempotent)
	_, err := suite.kv.DeleteTree("", nil)
	suite.Require().NoError(err, "unable to clear key value store")

	suite.set("NO_NAMESPACE_STRING", "hello")
	suite.set("NO_NAMESPACE_INT", "42")
	suite.set("FOO/EMPTY", "")
	suite.set("FOO/HTTP_HOST", "foo.example.com")
	suite.set("FOO/HTTP_PORT", "1234")
	suite.set("FOO/FLOAT", "12.345")
	suite.set("FOO/BOOL_TRUE", "true")
	suite.set("FOO/BOOL_TRUE_UPPER", "TRUE")
	suite.set("FOO/BOOL_FALSE", "false")
	suite.set("FOO/LABELS", "a, b,   c ,d ")
	suite.set("FOO/DURATION_1", "5m3s")
	suite.set("FOO/DURATION_2", "12h")
	suite.set("FOO/DATE", "2019-12-25")
	suite.set("FOO/DATE_TIME", "2019-12-25T12:00:05.0Z")
	suite.set("BAR/HTTP_HOST", "bar.example.com")
}

func (suite *ConsulSuite) set(key, value string) {
	_, err := suite.kv.Put(&api.KVPair{Key: key, Value: []byte(value)}, nil)
	suite.Require().NoError(err, "unable to write pair "+key+"="+value)
}

func (suite *ConsulSuite) TestFactoryValidation() {
	_, err := consul.NewSource(
		configify.Address(consulTestEndpoint),
	)
	suite.Error(err, "should return an error: no context")

	_, err = consul.NewSource(
		configify.Context(context.TODO()),
	)
	suite.Error(err, "should return an error: no address")

	_, err = consul.NewSource(
		configify.Context(context.TODO()),
		configify.Address("ftp://moo.:random-junk-host:12938129381"),
	)
	suite.Error(err, "should return an error: bad address")

	// The consul client doesn't fail at this point. Your calls just won't work.
	_, err = consul.NewSource(
		configify.Context(context.TODO()),
		configify.Address(consulTestEndpoint),
		configify.Username("hello"),
		configify.Password("world"),
	)
	suite.NoError(err, "should not return an error when supplying bad credentials")
}

// TestWatcher makes sure that your registered watcher fires when a value is updated
// in the backend consul KV store.
func (suite *ConsulSuite) TestWatcher() {
	source, _ := consul.NewSource(
		configify.Context(suite.context),
		configify.Address(consulTestEndpoint),
		configify.RefreshInterval(1*time.Second),
	)

	// Make sure the initial value is correct
	value, _ := source.String("FOO/HTTP_HOST")
	suite.Equal("foo.example.com", value)

	wg := sync.WaitGroup{}
	wg.Add(1)

	source.Watch(func(s configify.Source) {
		value, _ := source.String("FOO/HTTP_HOST")
		suite.Equal("google.com", value)
		wg.Done()
	})

	// Update the value then wait for our handler to detect the update.
	suite.set("FOO/HTTP_HOST", "google.com")
	wg.Wait()
}

// TestRefreshDelay verifies that updates to the backend Consul store are not immediate, but
// happen after the configured refresh interval.
func (suite *ConsulSuite) TestRefreshDelay() {
	source, _ := consul.NewSource(
		configify.Context(suite.context),
		configify.Address(consulTestEndpoint),
		configify.RefreshInterval(1*time.Second),
	)

	// Read the initial value then change it in Consul
	value, _ := source.String("FOO/HTTP_HOST")
	suite.Equal("foo.example.com", value)
	suite.set("FOO/HTTP_HOST", "google.com")

	// Our updates are not immediate. It will take at least the "RefreshInterval" to
	// realize the new value for the key.
	value, _ = source.String("FOO/HTTP_HOST")
	suite.Equal("foo.example.com", value)

	// Now that another refresh cycle has occurred, the new value is available.
	time.Sleep(2 * time.Second)
	value, _ = source.String("FOO/HTTP_HOST")
	suite.Equal("google.com", value)

	// Since we didn't change it, the next refresh cycle should be the same value.
	time.Sleep(2 * time.Second)
	value, _ = source.String("FOO/HTTP_HOST")
	suite.Equal("google.com", value)
}

// TestRefreshFailure ensures that we can still create a valid store even if we can't
// establish a real connection to Consul. You get blank values
func (suite *ConsulSuite) TestRefreshFailure() {
	// The consul.NewClient() function only barfs if it can't recognize the protocol. It
	// doesn't do anything if the host is bad, unfortunately.
	source, _ := consul.NewSource(
		configify.Context(suite.context),
		configify.Address("asldjfaslkdjf"),
		configify.RefreshInterval(1*time.Second),
	)

	text, ok := source.String("FOO/HTTP_HOST")
	suite.Equal("", text)
	suite.False(ok)

	number, ok := source.Int16("FOO/HTTP_PORT")
	suite.Equal(int16(0), number)
	suite.False(ok)
}

// TestCancelContext ensures that we stop listening for updates in Consul when the
// underlying context has expired.
func (suite *ConsulSuite) TestCancelContext() {
	source, _ := consul.NewSource(
		configify.Context(suite.context),
		configify.Address(consulTestEndpoint),
		configify.RefreshInterval(2*time.Second),
	)

	// We should have the initial values loaded at this point, so stop listening and update Consul
	suite.contextCancel()
	suite.set("FOO/HTTP_HOST", "google.com")

	// Wait for the next refresh to pass and make sure that it's still the old value
	time.Sleep(3 * time.Second)
	text, _ := source.String("FOO/HTTP_HOST")
	suite.Equal("foo.example.com", text)
}

func (suite *ConsulSuite) TestOptions() {
	suite.Equal(suite.Source.Options().Namespace.Name, "FOO")
	suite.Equal(suite.Source.Options().Namespace.Delimiter, "/")
}

func (suite *ConsulSuite) TestString() {
	// Good values we can parse
	suite.ExpectString("HTTP_HOST", "foo.example.com", true)
	suite.ExpectString("HTTP_PORT", "1234", true)

	// Keys not in our namespace
	suite.ExpectString("NO_NAMESPACE_STRING", "", false)
	suite.ExpectString("ASDF", "", false)
}

func (suite *ConsulSuite) TestStringSlice() {
	// Good values we can parse
	suite.ExpectStringSlice("LABELS", []string{"a", "b", "c", "d"}, true)
	suite.ExpectStringSlice("HTTP_PORT", []string{"1234"}, true)
	suite.ExpectStringSlice("FLOAT", []string{"12.345"}, true)

	// Keys not in our namespace
	suite.ExpectStringSlice("NO_NAMESPACE_STRING", nil, false)
	suite.ExpectStringSlice("ASDF", nil, false)
}

func (suite *ConsulSuite) TestInt() {
	// Good values we can parse
	suite.ExpectInt("HTTP_PORT", 1234, true)
	suite.ExpectInt("FLOAT", 12, true)

	// Values that exist but don't parse to this type
	suite.ExpectInt("LABELS", 0, false)
	suite.ExpectInt("HTTP_HOST", 0, false)

	// Keys not in our namespace
	suite.ExpectInt("NO_NAMESPACE_STRING", 0, false)
	suite.ExpectInt("ASDF", 0, false)
}

func (suite *ConsulSuite) TestInt8() {
	// Good values we can parse
	suite.ExpectInt8("HTTP_PORT", int8(-46), true) // overflows so these are the bits left
	suite.ExpectInt8("FLOAT", int8(12), true)

	// Values that exist but don't parse to this type
	suite.ExpectInt8("LABELS", int8(0), false)
	suite.ExpectInt8("HTTP_HOST", int8(0), false)

	// Keys not in our namespace
	suite.ExpectInt8("NO_NAMESPACE_STRING", int8(0), false)
	suite.ExpectInt8("ASDF", int8(0), false)
}

func (suite *ConsulSuite) TestInt16() {
	// Good values we can parse
	suite.ExpectInt16("HTTP_PORT", int16(1234), true)
	suite.ExpectInt16("FLOAT", int16(12), true)

	// Values that exist but don't parse to this type
	suite.ExpectInt16("LABELS", int16(0), false)
	suite.ExpectInt16("HTTP_HOST", int16(0), false)

	// Keys not in our namespace
	suite.ExpectInt16("NO_NAMESPACE_STRING", int16(0), false)
	suite.ExpectInt16("ASDF", int16(0), false)
}

func (suite *ConsulSuite) TestInt32() {
	// Good values we can parse
	suite.ExpectInt32("HTTP_PORT", int32(1234), true)
	suite.ExpectInt32("FLOAT", int32(12), true)

	// Values that exist but don't parse to this type
	suite.ExpectInt32("LABELS", int32(0), false)
	suite.ExpectInt32("HTTP_HOST", int32(0), false)

	// Keys not in our namespace
	suite.ExpectInt32("NO_NAMESPACE_STRING", int32(0), false)
	suite.ExpectInt32("ASDF", int32(0), false)
}

func (suite *ConsulSuite) TestInt64() {
	// Good values we can parse
	suite.ExpectInt64("HTTP_PORT", int64(1234), true)
	suite.ExpectInt64("FLOAT", int64(12), true)

	// Values that exist but don't parse to this type
	suite.ExpectInt64("LABELS", int64(0), false)
	suite.ExpectInt64("HTTP_HOST", int64(0), false)

	// Keys not in our namespace
	suite.ExpectInt64("NO_NAMESPACE_STRING", int64(0), false)
	suite.ExpectInt64("ASDF", int64(0), false)
}

func (suite *ConsulSuite) TestUint() {
	// Good values we can parse
	suite.ExpectUint("HTTP_PORT", uint(1234), true)
	suite.ExpectUint("FLOAT", uint(12), true)

	// Values that exist but don't parse to this type
	suite.ExpectUint("LABELS", uint(0), false)
	suite.ExpectUint("HTTP_HOST", uint(0), false)

	// Keys not in our namespace
	suite.ExpectUint("NO_NAMESPACE_STRING", uint(0), false)
	suite.ExpectUint("ASDF", uint(0), false)
}

func (suite *ConsulSuite) TestUint8() {
	// Good values we can parse
	suite.ExpectUint8("HTTP_PORT", uint8(210), true) // overflows so these are the bits left
	suite.ExpectUint8("FLOAT", uint8(12), true)

	// Values that exist but don't parse to this type
	suite.ExpectUint8("LABELS", uint8(0), false)
	suite.ExpectUint8("HTTP_HOST", uint8(0), false)

	// Keys not in our namespace
	suite.ExpectUint8("NO_NAMESPACE_STRING", uint8(0), false)
	suite.ExpectUint8("ASDF", uint8(0), false)
}

func (suite *ConsulSuite) TestUint16() {
	// Good values we can parse
	suite.ExpectUint16("HTTP_PORT", uint16(1234), true)
	suite.ExpectUint16("FLOAT", uint16(12), true)

	// Values that exist but don't parse to this type
	suite.ExpectUint16("LABELS", uint16(0), false)
	suite.ExpectUint16("HTTP_HOST", uint16(0), false)

	// Keys not in our namespace
	suite.ExpectUint16("NO_NAMESPACE_STRING", uint16(0), false)
	suite.ExpectUint16("ASDF", uint16(0), false)
}

func (suite *ConsulSuite) TestUint32() {
	// Good values we can parse
	suite.ExpectUint32("HTTP_PORT", uint32(1234), true)
	suite.ExpectUint32("FLOAT", uint32(12), true)

	// Values that exist but don't parse to this type
	suite.ExpectUint32("LABELS", uint32(0), false)
	suite.ExpectUint32("HTTP_HOST", uint32(0), false)

	// Keys not in our namespace
	suite.ExpectUint32("NO_NAMESPACE_STRING", uint32(0), false)
	suite.ExpectUint32("ASDF", uint32(0), false)
}

func (suite *ConsulSuite) TestUint64() {
	// Good values we can parse
	suite.ExpectUint64("HTTP_PORT", uint64(1234), true)
	suite.ExpectUint64("FLOAT", uint64(12), true)

	// Values that exist but don't parse to this type
	suite.ExpectUint64("LABELS", uint64(0), false)
	suite.ExpectUint64("HTTP_HOST", uint64(0), false)

	// Keys not in our namespace
	suite.ExpectUint64("NO_NAMESPACE_STRING", uint64(0), false)
	suite.ExpectUint64("ASDF", uint64(0), false)
}

func (suite *ConsulSuite) TestFloat32() {
	// Good values we can parse
	suite.ExpectFloat32("HTTP_PORT", float32(1234), true)
	suite.ExpectFloat32("FLOAT", float32(12.345), true)

	// Values that exist but don't parse to this type
	suite.ExpectFloat32("LABELS", float32(0), false)
	suite.ExpectFloat32("HTTP_HOST", float32(0), false)

	// Keys not in our namespace
	suite.ExpectFloat32("NO_NAMESPACE_STRING", float32(0), false)
	suite.ExpectFloat32("ASDF", float32(0), false)
}

func (suite *ConsulSuite) TestFloat64() {
	// Good values we can parse
	suite.ExpectFloat64("HTTP_PORT", float64(1234), true)
	suite.ExpectFloat64("FLOAT", float64(12.345), true)

	// Values that exist but don't parse to this type
	suite.ExpectFloat64("LABELS", float64(0), false)
	suite.ExpectFloat64("HTTP_HOST", float64(0), false)

	// Keys not in our namespace
	suite.ExpectFloat64("NO_NAMESPACE_STRING", float64(0), false)
	suite.ExpectFloat64("ASDF", float64(0), false)
}

func (suite *ConsulSuite) TestBool() {
	// Good values we can parse
	suite.ExpectBool("BOOL_TRUE", true, true)
	suite.ExpectBool("BOOL_TRUE_UPPER", true, true)
	suite.ExpectBool("BOOL_FALSE", false, true)

	// Values that exist but don't parse to this type
	suite.ExpectBool("LABELS", false, false)
	suite.ExpectBool("HTTP_HOST", false, false)

	// Keys not in our namespace
	suite.ExpectBool("NO_NAMESPACE_STRING", false, false)
	suite.ExpectBool("ASDF", false, false)
}

func (suite *ConsulSuite) TestDuration() {
	// Good values we can parse
	suite.ExpectDuration("DURATION_1", 5*time.Minute+3*time.Second, true)
	suite.ExpectDuration("DURATION_2", 12*time.Hour, true)

	// Values that exist but don't parse to this type
	suite.ExpectDuration("LABELS", time.Duration(0), false)
	suite.ExpectDuration("HTTP_PORT", time.Duration(0), false)

	// Keys not in our namespace
	suite.ExpectDuration("NO_NAMESPACE_STRING", time.Duration(0), false)
	suite.ExpectDuration("ASDF", time.Duration(0), false)
}

func (suite *ConsulSuite) TestTime() {
	// Good values we can parse
	suite.ExpectTime("DATE", time.Date(2019, 12, 25, 0, 0, 0, 0, time.UTC), true)
	suite.ExpectTime("DATE_TIME", time.Date(2019, 12, 25, 12, 0, 5, 0, time.UTC), true)

	// Values that exist but don't parse to this type
	suite.ExpectTime("LABELS", time.Time{}, false)
	suite.ExpectTime("HTTP_PORT", time.Time{}, false)

	// Keys not in our namespace
	suite.ExpectTime("NO_NAMESPACE_STRING", time.Time{}, false)
	suite.ExpectTime("ASDF", time.Time{}, false)
}
