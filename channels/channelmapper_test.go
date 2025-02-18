//  Copyright (c) 2012 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file
//  except in compliance with the License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software distributed under the
//  License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
//  either express or implied. See the License for the specific language governing permissions
//  and limitations under the License.

package channels

import (
	"testing"

	"github.com/couchbase/sync_gateway/base"
	goassert "github.com/couchbaselabs/go.assert"
	"github.com/robertkrimen/otto"
	"github.com/robertkrimen/otto/underscore"
	"github.com/stretchr/testify/assert"
)

func init() {
	underscore.Disable() // It really slows down unit tests (by making otto.New take a lot longer)
}

func parse(jsonStr string) map[string]interface{} {
	var parsed map[string]interface{}
	base.JSONUnmarshal([]byte(jsonStr), &parsed)
	return parsed
}

var noUser = map[string]interface{}{"name": nil, "channels": []string{}}

func TestOttoValueToStringArray(t *testing.T) {
	// Test for https://github.com/robertkrimen/otto/issues/24
	value, _ := otto.New().ToValue([]string{"foo", "bar", "baz"})
	strings := ottoValueToStringArray(value)
	goassert.DeepEquals(t, strings, []string{"foo", "bar", "baz"})
}

// verify that our version of Otto treats JSON parsed arrays like real arrays
func TestJavaScriptWorks(t *testing.T) {
	mapper := NewChannelMapper(`function(doc) {channel(doc.x.concat(doc.y));}`)
	res, err := mapper.MapToChannelsAndAccess(parse(`{"x":["abc"],"y":["xyz"]}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Channels, SetOf(t, "abc", "xyz"))
}

// Just verify that the calls to the channel() fn show up in the output channel list.
func TestSyncFunction(t *testing.T) {
	mapper := NewChannelMapper(`function(doc) {channel("foo", "bar"); channel("baz")}`)
	res, err := mapper.MapToChannelsAndAccess(parse(`{"channels": []}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Channels, SetOf(t, "foo", "bar", "baz"))
}

// Just verify that the calls to the access() fn show up in the output channel list.
func TestAccessFunction(t *testing.T) {
	mapper := NewChannelMapper(`function(doc) {access("foo", "bar"); access("foo", "baz")}`)
	res, err := mapper.MapToChannelsAndAccess(parse(`{}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Access, AccessMap{"foo": SetOf(t, "bar", "baz")})
}

// Just verify that the calls to the channel() fn show up in the output channel list.
func TestSyncFunctionTakesArray(t *testing.T) {
	mapper := NewChannelMapper(`function(doc) {channel(["foo", "bar ok","baz"])}`)
	res, err := mapper.MapToChannelsAndAccess(parse(`{"channels": []}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Channels, SetOf(t, "foo", "bar ok", "baz"))
}

// Calling channel() with an invalid channel name should return an error.
func TestSyncFunctionRejectsInvalidChannels(t *testing.T) {
	mapper := NewChannelMapper(`function(doc) {channel(["foo", "bad,name","baz"])}`)
	_, err := mapper.MapToChannelsAndAccess(parse(`{"channels": []}`), `{}`, noUser)
	goassert.True(t, err != nil)
}

// Calling access() with an invalid channel name should return an error.
func TestAccessFunctionRejectsInvalidChannels(t *testing.T) {
	mapper := NewChannelMapper(`function(doc) {access("foo", "bad,name");}`)
	_, err := mapper.MapToChannelsAndAccess(parse(`{}`), `{}`, noUser)
	goassert.True(t, err != nil)
}

// Just verify that the calls to the access() fn show up in the output channel list.
func TestAccessFunctionTakesArrayOfUsers(t *testing.T) {
	mapper := NewChannelMapper(`function(doc) {access(["foo","bar","baz"], "ginger")}`)
	res, err := mapper.MapToChannelsAndAccess(parse(`{}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Access, AccessMap{"bar": SetOf(t, "ginger"), "baz": SetOf(t, "ginger"), "foo": SetOf(t, "ginger")})
}

// Just verify that the calls to the access() fn show up in the output channel list.
func TestAccessFunctionTakesArrayOfChannels(t *testing.T) {
	mapper := NewChannelMapper(`function(doc) {access("lee", ["ginger", "earl_grey", "green"])}`)
	res, err := mapper.MapToChannelsAndAccess(parse(`{}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Access, AccessMap{"lee": SetOf(t, "ginger", "earl_grey", "green")})
}

func TestAccessFunctionTakesArrayOfChannelsAndUsers(t *testing.T) {
	mapper := NewChannelMapper(`function(doc) {access(["lee", "nancy"], ["ginger", "earl_grey", "green"])}`)
	res, err := mapper.MapToChannelsAndAccess(parse(`{}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Access["lee"], SetOf(t, "ginger", "earl_grey", "green"))
	goassert.DeepEquals(t, res.Access["nancy"], SetOf(t, "ginger", "earl_grey", "green"))
}

func TestAccessFunctionTakesEmptyArrayUser(t *testing.T) {
	mapper := NewChannelMapper(`function(doc) {access([], ["ginger", "earl grey", "green"])}`)
	res, err := mapper.MapToChannelsAndAccess(parse(`{}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Access, AccessMap{})
}

func TestAccessFunctionTakesEmptyArrayChannels(t *testing.T) {
	mapper := NewChannelMapper(`function(doc) {access("lee", [])}`)
	res, err := mapper.MapToChannelsAndAccess(parse(`{}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Access, AccessMap{})
}

func TestAccessFunctionTakesNullUser(t *testing.T) {
	mapper := NewChannelMapper(`function(doc) {access(null, ["ginger", "earl grey", "green"])}`)
	res, err := mapper.MapToChannelsAndAccess(parse(`{}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Access, AccessMap{})
}

func TestAccessFunctionTakesNullChannels(t *testing.T) {
	mapper := NewChannelMapper(`function(doc) {access("lee", null)}`)
	res, err := mapper.MapToChannelsAndAccess(parse(`{}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Access, AccessMap{})
}

func TestAccessFunctionTakesNonChannelsInArray(t *testing.T) {
	mapper := NewChannelMapper(`function(doc) {access("lee", ["ginger", null, 5])}`)
	res, err := mapper.MapToChannelsAndAccess(parse(`{}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Access, AccessMap{"lee": SetOf(t, "ginger")})
}

func TestAccessFunctionTakesUndefinedUser(t *testing.T) {
	mapper := NewChannelMapper(`function(doc) {var x = {}; access(x.nothing, ["ginger", "earl grey", "green"])}`)
	res, err := mapper.MapToChannelsAndAccess(parse(`{}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Access, AccessMap{})
}

// Just verify that the calls to the role() fn show up in the output. (It shares a common
// implementation with access(), so most of the above tests also apply to it.)
func TestRoleFunction(t *testing.T) {
	mapper := NewChannelMapper(`function(doc) {role(["foo","bar","baz"], "role:froods")}`)
	res, err := mapper.MapToChannelsAndAccess(parse(`{}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Roles, AccessMap{"bar": SetOf(t, "froods"), "baz": SetOf(t, "froods"), "foo": SetOf(t, "froods")})
}

// Now just make sure the input comes through intact
func TestInputParse(t *testing.T) {
	mapper := NewChannelMapper(`function(doc) {channel(doc.channel);}`)
	res, err := mapper.MapToChannelsAndAccess(parse(`{"channel": "foo"}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Channels, SetOf(t, "foo"))
}

// A more realistic example
func TestDefaultChannelMapper(t *testing.T) {
	mapper := NewDefaultChannelMapper()
	res, err := mapper.MapToChannelsAndAccess(parse(`{"channels": ["foo", "bar", "baz"]}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Channels, SetOf(t, "foo", "bar", "baz"))

	res, err = mapper.MapToChannelsAndAccess(parse(`{"x": "y"}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Channels, base.Set{})
}

// Empty/no-op channel mapper fn
func TestEmptyChannelMapper(t *testing.T) {
	mapper := NewChannelMapper(`function(doc) {}`)
	res, err := mapper.MapToChannelsAndAccess(parse(`{"channels": ["foo", "bar", "baz"]}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Channels, base.Set{})
}

// channel mapper fn that uses _ underscore JS library
func TestChannelMapperUnderscoreLib(t *testing.T) {
	underscore.Enable() // It really slows down unit tests (by making otto.New take a lot longer)
	defer underscore.Disable()
	mapper := NewChannelMapper(`function(doc) {channel(_.first(doc.channels));}`)
	res, err := mapper.MapToChannelsAndAccess(parse(`{"channels": ["foo", "bar", "baz"]}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Channels, SetOf(t, "foo"))
}

// Validation by calling reject()
func TestChannelMapperReject(t *testing.T) {
	mapper := NewChannelMapper(`function(doc) {reject(403, "bad");}`)
	res, err := mapper.MapToChannelsAndAccess(parse(`{"channels": ["foo", "bar", "baz"]}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Rejection, base.HTTPErrorf(403, "bad"))
}

// Rejection by calling throw()
func TestChannelMapperThrow(t *testing.T) {
	mapper := NewChannelMapper(`function(doc) {throw({forbidden:"bad"});}`)
	res, err := mapper.MapToChannelsAndAccess(parse(`{"channels": ["foo", "bar", "baz"]}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Rejection, base.HTTPErrorf(403, "bad"))
}

// Test other runtime exception
func TestChannelMapperException(t *testing.T) {
	mapper := NewChannelMapper(`function(doc) {(nil)[5];}`)
	_, err := mapper.MapToChannelsAndAccess(parse(`{"channels": ["foo", "bar", "baz"]}`), `{}`, noUser)
	goassert.True(t, err != nil)
}

// Test the public API
func TestPublicChannelMapper(t *testing.T) {
	mapper := NewChannelMapper(`function(doc) {channel(doc.channels);}`)
	output, err := mapper.MapToChannelsAndAccess(parse(`{"channels": ["foo", "bar", "baz"]}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, output.Channels, SetOf(t, "foo", "bar", "baz"))
}

// Test the userCtx name parameter
func TestCheckUser(t *testing.T) {
	mapper := NewChannelMapper(`function(doc, oldDoc) {
			requireUser(doc.owner);
		}`)
	var sally = map[string]interface{}{"name": "sally", "channels": []string{}}
	res, err := mapper.MapToChannelsAndAccess(parse(`{"owner": "sally"}`), `{}`, sally)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Rejection, nil)

	var linus = map[string]interface{}{"name": "linus", "channels": []string{}}
	res, err = mapper.MapToChannelsAndAccess(parse(`{"owner": "sally"}`), `{}`, linus)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Rejection, base.HTTPErrorf(403, base.SyncFnErrorWrongUser))

	res, err = mapper.MapToChannelsAndAccess(parse(`{"owner": "sally"}`), `{}`, nil)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Rejection, nil)
}

// Test the userCtx name parameter with a list
func TestCheckUserArray(t *testing.T) {
	mapper := NewChannelMapper(`function(doc, oldDoc) {
			requireUser(doc.owners);
		}`)
	var sally = map[string]interface{}{"name": "sally", "channels": []string{}}
	res, err := mapper.MapToChannelsAndAccess(parse(`{"owners": ["sally", "joe"]}`), `{}`, sally)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Rejection, nil)

	var linus = map[string]interface{}{"name": "linus", "channels": []string{}}
	res, err = mapper.MapToChannelsAndAccess(parse(`{"owners": ["sally", "joe"]}`), `{}`, linus)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Rejection, base.HTTPErrorf(403, base.SyncFnErrorWrongUser))

	res, err = mapper.MapToChannelsAndAccess(parse(`{"owners": ["sally"]}`), `{}`, nil)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Rejection, nil)
}

// Test the userCtx role parameter
func TestCheckRole(t *testing.T) {
	mapper := NewChannelMapper(`function(doc, oldDoc) {
			requireRole(doc.role);
		}`)
	var sally = map[string]interface{}{"name": "sally", "roles": map[string]int{"girl": 1, "5yo": 1}}
	res, err := mapper.MapToChannelsAndAccess(parse(`{"role": "girl"}`), `{}`, sally)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Rejection, nil)

	var linus = map[string]interface{}{"name": "linus", "roles": []string{"boy", "musician"}}
	res, err = mapper.MapToChannelsAndAccess(parse(`{"role": "girl"}`), `{}`, linus)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Rejection, base.HTTPErrorf(403, base.SyncFnErrorMissingRole))

	res, err = mapper.MapToChannelsAndAccess(parse(`{"role": "girl"}`), `{}`, nil)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Rejection, nil)
}

// Test the userCtx role parameter with a list
func TestCheckRoleArray(t *testing.T) {
	mapper := NewChannelMapper(`function(doc, oldDoc) {
			requireRole(doc.roles);
		}`)
	var sally = map[string]interface{}{"name": "sally", "roles": map[string]int{"girl": 1, "5yo": 1}}
	res, err := mapper.MapToChannelsAndAccess(parse(`{"roles": ["kid","girl"]}`), `{}`, sally)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Rejection, nil)

	var linus = map[string]interface{}{"name": "linus", "roles": map[string]int{"boy": 1, "musician": 1}}
	res, err = mapper.MapToChannelsAndAccess(parse(`{"roles": ["girl"]}`), `{}`, linus)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Rejection, base.HTTPErrorf(403, base.SyncFnErrorMissingRole))

	res, err = mapper.MapToChannelsAndAccess(parse(`{"roles": ["girl"]}`), `{}`, nil)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Rejection, nil)
}

// Test the userCtx.channels parameter
func TestCheckAccess(t *testing.T) {
	mapper := NewChannelMapper(`function(doc, oldDoc) {
		requireAccess(doc.channel)
	}`)
	var sally = map[string]interface{}{"name": "sally", "roles": []string{"girl", "5yo"}, "channels": []string{"party", "school"}}
	res, err := mapper.MapToChannelsAndAccess(parse(`{"channel": "party"}`), `{}`, sally)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Rejection, nil)

	var linus = map[string]interface{}{"name": "linus", "roles": []string{"boy", "musician"}, "channels": []string{"party", "school"}}
	res, err = mapper.MapToChannelsAndAccess(parse(`{"channel": "work"}`), `{}`, linus)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Rejection, base.HTTPErrorf(403, base.SyncFnErrorMissingChannelAccess))

	res, err = mapper.MapToChannelsAndAccess(parse(`{"channel": "magic"}`), `{}`, nil)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Rejection, nil)
}

// Test the userCtx.channels parameter with a list
func TestCheckAccessArray(t *testing.T) {
	mapper := NewChannelMapper(`function(doc, oldDoc) {
		requireAccess(doc.channels)
	}`)
	var sally = map[string]interface{}{"name": "sally", "roles": []string{"girl", "5yo"}, "channels": []string{"party", "school"}}
	res, err := mapper.MapToChannelsAndAccess(parse(`{"channels": ["swim","party"]}`), `{}`, sally)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Rejection, nil)

	var linus = map[string]interface{}{"name": "linus", "roles": []string{"boy", "musician"}, "channels": []string{"party", "school"}}
	res, err = mapper.MapToChannelsAndAccess(parse(`{"channels": ["work"]}`), `{}`, linus)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Rejection, base.HTTPErrorf(403, base.SyncFnErrorMissingChannelAccess))

	res, err = mapper.MapToChannelsAndAccess(parse(`{"channels": ["magic"]}`), `{}`, nil)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, res.Rejection, nil)
}

// Test changing the function
func TestSetFunction(t *testing.T) {
	mapper := NewChannelMapper(`function(doc) {channel(doc.channels);}`)
	output, err := mapper.MapToChannelsAndAccess(parse(`{"channels": ["foo", "bar", "baz"]}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	changed, err := mapper.SetFunction(`function(doc) {channel("all");}`)
	assert.True(t, changed, "SetFunction failed")
	assert.NoError(t, err, "SetFunction failed")
	output, err = mapper.MapToChannelsAndAccess(parse(`{"channels": ["foo", "bar", "baz"]}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, output.Channels, SetOf(t, "all"))
}

// Test that expiry function sets the expiry property
func TestExpiryFunction(t *testing.T) {
	mapper := NewChannelMapper(`function(doc) {expiry(doc.expiry);}`)
	res1, err := mapper.MapToChannelsAndAccess(parse(`{"expiry":100}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess error")
	goassert.DeepEquals(t, *res1.Expiry, uint32(100))

	res2, err := mapper.MapToChannelsAndAccess(parse(`{"expiry":"500"}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess error")
	goassert.DeepEquals(t, *res2.Expiry, uint32(500))

	res_stringDate, err := mapper.MapToChannelsAndAccess(parse(`{"expiry":"2105-01-01T00:00:00.000+00:00"}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess error")
	goassert.DeepEquals(t, *res_stringDate.Expiry, uint32(4260211200))

	// Validate invalid expiry values log warning and don't set expiry
	res3, err := mapper.MapToChannelsAndAccess(parse(`{"expiry":"abc"}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess error for expiry:abc")
	goassert.True(t, res3.Expiry == nil)

	// Invalid: non-numeric
	res4, err := mapper.MapToChannelsAndAccess(parse(`{"expiry":["100", "200"]}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess error for expiry as array")
	goassert.True(t, res4.Expiry == nil)

	// Invalid: negative value
	res5, err := mapper.MapToChannelsAndAccess(parse(`{"expiry":-100}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess error for expiry as negative value")
	goassert.True(t, res5.Expiry == nil)

	// Invalid - larger than uint32
	res6, err := mapper.MapToChannelsAndAccess(parse(`{"expiry":123456789012345}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess error for expiry > unit32")
	goassert.True(t, res6.Expiry == nil)

	// Invalid - non-unix date
	resInvalidDate, err := mapper.MapToChannelsAndAccess(parse(`{"expiry":"1805-01-01T00:00:00.000+00:00"}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess error for expiry:1805-01-01T00:00:00.000+00:00")
	goassert.True(t, resInvalidDate.Expiry == nil)

	// No expiry specified
	res7, err := mapper.MapToChannelsAndAccess(parse(`{"value":5}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess error for expiry not specified")
	goassert.True(t, res7.Expiry == nil)
}

func TestExpiryFunctionConstantValue(t *testing.T) {
	mapper := NewChannelMapper(`function(doc) {expiry(100);}`)
	res1, err := mapper.MapToChannelsAndAccess(parse(`{}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess error")
	goassert.DeepEquals(t, *res1.Expiry, uint32(100))

	mapper = NewChannelMapper(`function(doc) {expiry("500");}`)
	res2, err := mapper.MapToChannelsAndAccess(parse(`{}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess error")
	goassert.DeepEquals(t, *res2.Expiry, uint32(500))

	mapper = NewChannelMapper(`function(doc) {expiry("2105-01-01T00:00:00.000+00:00");}`)
	res_stringDate, err := mapper.MapToChannelsAndAccess(parse(`{}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess error")
	goassert.DeepEquals(t, *res_stringDate.Expiry, uint32(4260211200))

	// Validate invalid expiry values log warning and don't set expiry
	mapper = NewChannelMapper(`function(doc) {expiry("abc");}`)
	res3, err := mapper.MapToChannelsAndAccess(parse(`{}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess error for expiry:abc")
	goassert.True(t, res3.Expiry == nil)

	// Invalid: non-numeric
	mapper = NewChannelMapper(`function(doc) {expiry(["100", "200"]);}`)
	res4, err := mapper.MapToChannelsAndAccess(parse(`{}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess error for expiry as array")
	goassert.True(t, res4.Expiry == nil)

	// Invalid: negative value
	mapper = NewChannelMapper(`function(doc) {expiry(-100);}`)
	res5, err := mapper.MapToChannelsAndAccess(parse(`{}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess error for expiry as negative value")
	goassert.True(t, res5.Expiry == nil)

	// Invalid - larger than uint32
	mapper = NewChannelMapper(`function(doc) {expiry(123456789012345);}`)
	res6, err := mapper.MapToChannelsAndAccess(parse(`{}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess error for expiry as > unit32")
	goassert.True(t, res6.Expiry == nil)

	// Invalid - non-unix date
	mapper = NewChannelMapper(`function(doc) {expiry("1805-01-01T00:00:00.000+00:00");}`)
	resInvalidDate, err := mapper.MapToChannelsAndAccess(parse(`{}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess error for expiry:1805-01-01T00:00:00.000+00:00")
	goassert.True(t, resInvalidDate.Expiry == nil)

	// No expiry specified
	mapper = NewChannelMapper(`function(doc) {expiry();}`)
	res7, err := mapper.MapToChannelsAndAccess(parse(`{}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess error for expiry not specified")
	goassert.True(t, res7.Expiry == nil)
}

// Test that expiry function when invoked more than once by sync function
func TestExpiryFunctionMultipleInvocation(t *testing.T) {
	mapper := NewChannelMapper(`function(doc) {expiry(doc.expiry); expiry(doc.secondExpiry)}`)
	res1, err := mapper.MapToChannelsAndAccess(parse(`{"expiry":100}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, *res1.Expiry, uint32(100))

	res2, err := mapper.MapToChannelsAndAccess(parse(`{"expiry":"500"}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess failed")
	goassert.DeepEquals(t, *res2.Expiry, uint32(500))

	// Validate invalid expiry values log warning and don't set expiry
	res3, err := mapper.MapToChannelsAndAccess(parse(`{"expiry":"abc"}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess filed for expiry:abc")
	goassert.True(t, res3.Expiry == nil)

	// Invalid: non-numeric
	res4, err := mapper.MapToChannelsAndAccess(parse(`{"expiry":["100", "200"]}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess filed for expiry as array")
	goassert.True(t, res4.Expiry == nil)

	// Invalid: negative value
	res5, err := mapper.MapToChannelsAndAccess(parse(`{"expiry":-100}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess filed for expiry as array")
	goassert.True(t, res5.Expiry == nil)

	// Invalid - larger than uint32
	res6, err := mapper.MapToChannelsAndAccess(parse(`{"expiry":123456789012345}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess filed for expiry as array")
	goassert.True(t, res6.Expiry == nil)

	// No expiry specified
	res7, err := mapper.MapToChannelsAndAccess(parse(`{"value":5}`), `{}`, noUser)
	assert.NoError(t, err, "MapToChannelsAndAccess filed for expiry as array")
	goassert.True(t, res7.Expiry == nil)
}

func TestChangedUsers(t *testing.T) {
	a := AccessMap{"alice": SetOf(t, "x", "y"), "bita": SetOf(t, "z"), "claire": SetOf(t, "w")}
	b := AccessMap{"alice": SetOf(t, "x", "z"), "bita": SetOf(t, "z"), "diana": SetOf(t, "w")}

	changes := map[string]bool{}
	ForChangedUsers(a, b, func(name string) {
		changes[name] = true
	})
	goassert.DeepEquals(t, changes, map[string]bool{"alice": true, "claire": true, "diana": true})
}
