//  Copyright (c) 2012 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file
//  except in compliance with the License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software distributed under the
//  License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
//  either express or implied. See the License for the specific language governing permissions
//  and limitations under the License.

package auth

import (
	"errors"
	"log"
	"sync"
	"testing"

	"github.com/couchbase/sync_gateway/base"
	ch "github.com/couchbase/sync_gateway/channels"
	goassert "github.com/couchbaselabs/go.assert"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func canSeeAllChannels(princ Principal, channels base.Set) bool {
	for channel := range channels {
		if !princ.CanSeeChannel(channel) {
			return false
		}
	}
	return true
}

func TestValidateGuestUser(t *testing.T) {

	gTestBucket := base.GetTestBucket(t)
	defer gTestBucket.Close()
	bucket := gTestBucket.Bucket
	auth := NewAuthenticator(bucket, nil)
	user, err := auth.NewUser("", "", nil)
	assert.True(t, user != nil)
	assert.True(t, err == nil)
}

func TestValidateUser(t *testing.T) {

	gTestBucket := base.GetTestBucket(t)
	defer gTestBucket.Close()
	auth := NewAuthenticator(gTestBucket.Bucket, nil)
	user, err := auth.NewUser("invalid:name", "", nil)
	assert.Equal(t, user, (User)(nil))
	assert.True(t, err != nil)
	user, err = auth.NewUser("ValidName", "", nil)
	assert.True(t, user != nil)
	assert.Equal(t, err, nil)
	user, err = auth.NewUser("ValidName", "letmein", nil)
	assert.True(t, user != nil)
	assert.Equal(t, err, nil)
}

func TestValidateRole(t *testing.T) {

	gTestBucket := base.GetTestBucket(t)
	defer gTestBucket.Close()
	auth := NewAuthenticator(gTestBucket.Bucket, nil)
	role, err := auth.NewRole("invalid:name", nil)
	assert.Equal(t, (User)(nil), role)
	assert.True(t, err != nil)
	role, err = auth.NewRole("ValidName", nil)
	assert.True(t, role != nil)
	assert.Equal(t, nil, err)
	role, err = auth.NewRole("ValidName", nil)
	assert.True(t, role != nil)
	assert.Equal(t, nil, err)
}

func TestValidateUserEmail(t *testing.T) {

	gTestBucket := base.GetTestBucket(t)
	defer gTestBucket.Close()
	auth := NewAuthenticator(gTestBucket.Bucket, nil)
	badEmails := []string{"", "foo", "foo@", "@bar", "foo @bar", "foo@.bar"}
	for _, e := range badEmails {
		assert.False(t, IsValidEmail(e))
	}
	goodEmails := []string{"foo@bar", "foo.99@bar.com", "f@bar.exampl-3.com."}
	for _, e := range goodEmails {
		assert.True(t, IsValidEmail(e))
	}
	user, _ := auth.NewUser("ValidName", "letmein", nil)
	assert.False(t, user.SetEmail("foo") == nil)
	assert.Equal(t, nil, user.SetEmail("foo@example.com"))
}

func TestUserPasswords(t *testing.T) {

	gTestBucket := base.GetTestBucket(t)
	defer gTestBucket.Close()
	auth := NewAuthenticator(gTestBucket.Bucket, nil)
	user, _ := auth.NewUser("me", "letmein", nil)
	assert.True(t, user.Authenticate("letmein"))
	assert.False(t, user.Authenticate("password"))
	assert.False(t, user.Authenticate(""))

	guest, _ := auth.NewUser("", "", nil)
	assert.True(t, guest.Authenticate(""))
	assert.False(t, guest.Authenticate("123456"))

	// Create a second user with the same password
	user2, _ := auth.NewUser("me", "letmein", nil)
	assert.True(t, user2.Authenticate("letmein"))
	assert.False(t, user2.Authenticate("password"))
	assert.True(t, user.Authenticate("letmein"))
	assert.False(t, user.Authenticate("password"))
}

func TestSerializeUser(t *testing.T) {

	gTestBucket := base.GetTestBucket(t)
	defer gTestBucket.Close()
	auth := NewAuthenticator(gTestBucket.Bucket, nil)
	user, _ := auth.NewUser("me", "letmein", ch.SetOf(t, "me", "public"))
	require.NoError(t, user.SetEmail("foo@example.com"))
	encoded, _ := base.JSONMarshal(user)
	assert.True(t, encoded != nil)
	log.Printf("Marshaled User as: %s", encoded)

	resu := &userImpl{}
	err := base.JSONUnmarshal(encoded, resu)
	assert.True(t, err == nil)
	goassert.DeepEquals(t, resu.Name(), user.Name())
	goassert.DeepEquals(t, resu.Email(), user.Email())
	goassert.DeepEquals(t, resu.ExplicitChannels(), user.ExplicitChannels())
	assert.True(t, resu.Authenticate("letmein"))
	assert.False(t, resu.Authenticate("123456"))
}

func TestSerializeRole(t *testing.T) {

	gTestBucket := base.GetTestBucket(t)
	defer gTestBucket.Close()
	auth := NewAuthenticator(gTestBucket.Bucket, nil)
	role, _ := auth.NewRole("froods", ch.SetOf(t, "hoopy", "public"))
	encoded, _ := base.JSONMarshal(role)
	assert.True(t, encoded != nil)
	log.Printf("Marshaled Role as: %s", encoded)
	elor := &roleImpl{}
	err := base.JSONUnmarshal(encoded, elor)

	assert.True(t, err == nil)
	goassert.DeepEquals(t, elor.Name(), role.Name())
	goassert.DeepEquals(t, elor.ExplicitChannels(), role.ExplicitChannels())
}

func TestUserAccess(t *testing.T) {

	// User with no access:
	gTestBucket := base.GetTestBucket(t)
	defer gTestBucket.Close()
	auth := NewAuthenticator(gTestBucket.Bucket, nil)
	user, _ := auth.NewUser("foo", "password", nil)
	goassert.DeepEquals(t, user.ExpandWildCardChannel(ch.SetOf(t, "*")), ch.SetOf(t, "!"))
	assert.False(t, user.CanSeeChannel("x"))
	assert.True(t, canSeeAllChannels(user, ch.SetOf(t)))
	assert.False(t, canSeeAllChannels(user, ch.SetOf(t, "x")))
	assert.False(t, canSeeAllChannels(user, ch.SetOf(t, "x", "y")))
	assert.False(t, canSeeAllChannels(user, ch.SetOf(t, "*")))
	assert.False(t, user.AuthorizeAllChannels(ch.SetOf(t, "*")) == nil)
	assert.False(t, user.AuthorizeAnyChannel(ch.SetOf(t, "x", "y")) == nil)
	assert.False(t, user.AuthorizeAnyChannel(ch.SetOf(t)) == nil)

	// User with access to one channel:
	user.setChannels(ch.AtSequence(ch.SetOf(t, "x"), 1))
	goassert.DeepEquals(t, user.ExpandWildCardChannel(ch.SetOf(t, "*")), ch.SetOf(t, "x"))
	assert.True(t, canSeeAllChannels(user, ch.SetOf(t)))
	assert.True(t, canSeeAllChannels(user, ch.SetOf(t, "x")))
	assert.False(t, canSeeAllChannels(user, ch.SetOf(t, "x", "y")))
	assert.False(t, user.AuthorizeAllChannels(ch.SetOf(t, "x", "y")) == nil)
	assert.False(t, user.AuthorizeAllChannels(ch.SetOf(t, "*")) == nil)
	assert.True(t, user.AuthorizeAnyChannel(ch.SetOf(t, "x", "y")) == nil)
	assert.False(t, user.AuthorizeAnyChannel(ch.SetOf(t, "y")) == nil)
	assert.False(t, user.AuthorizeAnyChannel(ch.SetOf(t)) == nil)

	// User with access to one channel and one derived channel:
	user.setChannels(ch.AtSequence(ch.SetOf(t, "x", "z"), 1))
	goassert.DeepEquals(t, user.ExpandWildCardChannel(ch.SetOf(t, "*")), ch.SetOf(t, "x", "z"))
	goassert.DeepEquals(t, user.ExpandWildCardChannel(ch.SetOf(t, "x")), ch.SetOf(t, "x"))
	assert.True(t, canSeeAllChannels(user, ch.SetOf(t)))
	assert.True(t, canSeeAllChannels(user, ch.SetOf(t, "x")))
	assert.False(t, canSeeAllChannels(user, ch.SetOf(t, "x", "y")))
	assert.False(t, user.AuthorizeAllChannels(ch.SetOf(t, "x", "y")) == nil)
	assert.False(t, user.AuthorizeAllChannels(ch.SetOf(t, "*")) == nil)

	// User with access to two channels:
	user.setChannels(ch.AtSequence(ch.SetOf(t, "x", "z"), 1))
	goassert.DeepEquals(t, user.ExpandWildCardChannel(ch.SetOf(t, "*")), ch.SetOf(t, "x", "z"))
	goassert.DeepEquals(t, user.ExpandWildCardChannel(ch.SetOf(t, "x")), ch.SetOf(t, "x"))
	assert.True(t, canSeeAllChannels(user, ch.SetOf(t)))
	assert.True(t, canSeeAllChannels(user, ch.SetOf(t, "x")))
	assert.False(t, canSeeAllChannels(user, ch.SetOf(t, "x", "y")))
	assert.False(t, user.AuthorizeAllChannels(ch.SetOf(t, "x", "y")) == nil)
	assert.False(t, user.AuthorizeAllChannels(ch.SetOf(t, "*")) == nil)

	user.setChannels(ch.AtSequence(ch.SetOf(t, "x", "y"), 1))
	goassert.DeepEquals(t, user.ExpandWildCardChannel(ch.SetOf(t, "*")), ch.SetOf(t, "x", "y"))
	assert.True(t, canSeeAllChannels(user, ch.SetOf(t)))
	assert.True(t, canSeeAllChannels(user, ch.SetOf(t, "x")))
	assert.True(t, canSeeAllChannels(user, ch.SetOf(t, "x", "y")))
	assert.False(t, canSeeAllChannels(user, ch.SetOf(t, "x", "y", "z")))
	assert.True(t, user.AuthorizeAllChannels(ch.SetOf(t, "x", "y")) == nil)
	assert.False(t, user.AuthorizeAllChannels(ch.SetOf(t, "*")) == nil)

	// User with wildcard access:
	user.setChannels(ch.AtSequence(ch.SetOf(t, "*", "q"), 1))
	goassert.DeepEquals(t, user.ExpandWildCardChannel(ch.SetOf(t, "*")), ch.SetOf(t, "*", "q"))
	assert.True(t, user.CanSeeChannel("*"))
	assert.True(t, canSeeAllChannels(user, ch.SetOf(t)))
	assert.True(t, canSeeAllChannels(user, ch.SetOf(t, "x")))
	assert.True(t, canSeeAllChannels(user, ch.SetOf(t, "x", "y")))
	assert.True(t, user.AuthorizeAllChannels(ch.SetOf(t, "x", "y")) == nil)
	assert.True(t, user.AuthorizeAllChannels(ch.SetOf(t, "*")) == nil)
	assert.True(t, user.AuthorizeAnyChannel(ch.SetOf(t, "x")) == nil)
	assert.True(t, user.AuthorizeAnyChannel(ch.SetOf(t, "*")) == nil)
	assert.True(t, user.AuthorizeAnyChannel(ch.SetOf(t)) == nil)
}

func TestGetMissingUser(t *testing.T) {

	gTestBucket := base.GetTestBucket(t)
	defer gTestBucket.Close()
	auth := NewAuthenticator(gTestBucket.Bucket, nil)
	user, err := auth.GetUser("noSuchUser")
	assert.Equal(t, nil, err)
	assert.True(t, user == nil)
	user, err = auth.GetUserByEmail("noreply@example.com")
	assert.Equal(t, nil, err)
	assert.True(t, user == nil)
}

func TestGetMissingRole(t *testing.T) {

	gTestBucket := base.GetTestBucket(t)
	defer gTestBucket.Close()
	auth := NewAuthenticator(gTestBucket.Bucket, nil)
	role, err := auth.GetRole("noSuchRole")
	assert.Equal(t, nil, err)
	assert.True(t, role == nil)
}

func TestGetGuestUser(t *testing.T) {

	gTestBucket := base.GetTestBucket(t)
	defer gTestBucket.Close()
	auth := NewAuthenticator(gTestBucket.Bucket, nil)
	user, err := auth.GetUser("")
	assert.Equal(t, nil, err)
	goassert.DeepEquals(t, user, auth.defaultGuestUser())

}

func TestSaveUsers(t *testing.T) {

	gTestBucket := base.GetTestBucket(t)
	defer gTestBucket.Close()
	auth := NewAuthenticator(gTestBucket.Bucket, nil)
	user, _ := auth.NewUser("testUser", "password", ch.SetOf(t, "test"))
	err := auth.Save(user)
	assert.Equal(t, nil, err)

	user2, err := auth.GetUser("testUser")
	assert.Equal(t, nil, err)
	goassert.DeepEquals(t, user2, user)
}

func TestSaveRoles(t *testing.T) {
	gTestBucket := base.GetTestBucket(t)
	defer gTestBucket.Close()
	auth := NewAuthenticator(gTestBucket.Bucket, nil)
	role, _ := auth.NewRole("testRole", ch.SetOf(t, "test"))
	err := auth.Save(role)
	assert.Equal(t, nil, err)

	role2, err := auth.GetRole("testRole")
	assert.Equal(t, nil, err)
	goassert.DeepEquals(t, role2, role)
}

type mockComputer struct {
	channels     ch.TimedSet
	roles        ch.TimedSet
	roleChannels ch.TimedSet
	err          error
}

func (self *mockComputer) ComputeChannelsForPrincipal(p Principal) (ch.TimedSet, error) {
	switch p.(type) {
	case User:
		return self.channels, self.err
	case Role:
		return self.roleChannels, self.err
	default:
		return nil, self.err
	}
}

func (self *mockComputer) ComputeRolesForUser(User) (ch.TimedSet, error) {
	return self.roles, self.err
}

func (self *mockComputer) UseGlobalSequence() bool {
	return true
}

func TestRebuildUserChannels(t *testing.T) {

	gTestBucket := base.GetTestBucket(t)
	defer gTestBucket.Close()
	computer := mockComputer{channels: ch.AtSequence(ch.SetOf(t, "derived1", "derived2"), 1)}
	auth := NewAuthenticator(gTestBucket.Bucket, &computer)
	user, _ := auth.NewUser("testUser", "password", ch.SetOf(t, "explicit1"))
	user.setChannels(nil)
	err := auth.Save(user)
	assert.Equal(t, nil, err)

	user2, err := auth.GetUser("testUser")
	assert.Equal(t, nil, err)
	goassert.DeepEquals(t, user2.Channels(), ch.AtSequence(ch.SetOf(t, "explicit1", "derived1", "derived2", "!"), 1))
}

func TestRebuildRoleChannels(t *testing.T) {

	gTestBucket := base.GetTestBucket(t)
	defer gTestBucket.Close()
	computer := mockComputer{roleChannels: ch.AtSequence(ch.SetOf(t, "derived1", "derived2"), 1)}
	auth := NewAuthenticator(gTestBucket.Bucket, &computer)
	role, err := auth.NewRole("testRole", ch.SetOf(t, "explicit1"))
	assert.NoError(t, err)
	err = auth.InvalidateChannels(role)
	assert.Equal(t, nil, err)

	role2, err := auth.GetRole("testRole")
	assert.Equal(t, nil, err)
	assert.Equal(t, ch.AtSequence(ch.SetOf(t, "explicit1", "derived1", "derived2", "!"), 1), role2.Channels())
}

func TestRebuildChannelsError(t *testing.T) {

	gTestBucket := base.GetTestBucket(t)
	defer gTestBucket.Close()
	computer := mockComputer{}
	auth := NewAuthenticator(gTestBucket.Bucket, &computer)
	role, err := auth.NewRole("testRole2", ch.SetOf(t, "explicit1"))
	assert.NoError(t, err)
	assert.Equal(t, nil, auth.InvalidateChannels(role))

	computer.err = errors.New("I'm sorry, Dave.")

	role2, err := auth.GetRole("testRole2")
	assert.Nil(t, role2)
	assert.Equal(t, computer.err, err)
}

func TestRebuildUserRoles(t *testing.T) {

	gTestBucket := base.GetTestBucket(t)
	defer gTestBucket.Close()
	computer := mockComputer{roles: ch.AtSequence(base.SetOf("role1", "role2"), 3)}
	auth := NewAuthenticator(gTestBucket.Bucket, &computer)
	user, _ := auth.NewUser("testUser", "letmein", nil)
	user.SetExplicitRoles(ch.TimedSet{"role3": ch.NewVbSimpleSequence(1), "role1": ch.NewVbSimpleSequence(1)})
	err := auth.Save(user)
	assert.Equal(t, nil, err)

	// Retrieve the user, triggers initial build of roles
	user1, err := auth.GetUser("testUser")
	assert.Equal(t, nil, err)
	expected := ch.AtSequence(base.SetOf("role1", "role3"), 1)
	expected.AddChannel("role2", 3)
	goassert.DeepEquals(t, user1.RoleNames(), expected)

	// Invalidate the roles, triggers rebuild
	err = auth.InvalidateRoles(user1)
	assert.Equal(t, nil, err)

	user2, err := auth.GetUser("testUser")
	assert.Equal(t, nil, err)
	expected = ch.AtSequence(base.SetOf("role1", "role3"), 1)
	expected.AddChannel("role2", 3)
	goassert.DeepEquals(t, user2.RoleNames(), expected)
}

func TestRoleInheritance(t *testing.T) {
	// Create some roles:
	gTestBucket := base.GetTestBucket(t)
	defer gTestBucket.Close()
	auth := NewAuthenticator(gTestBucket.Bucket, nil)
	role, _ := auth.NewRole("square", ch.SetOf(t, "dull", "duller", "dullest"))
	assert.Equal(t, nil, auth.Save(role))
	role, _ = auth.NewRole("frood", ch.SetOf(t, "hoopy", "hoopier", "hoopiest"))
	assert.Equal(t, nil, auth.Save(role))

	user, _ := auth.NewUser("arthur", "password", ch.SetOf(t, "britain"))
	user.(*userImpl).setRolesSince(ch.TimedSet{"square": ch.NewVbSimpleSequence(0x3), "nonexistent": ch.NewVbSimpleSequence(0x42), "frood": ch.NewVbSimpleSequence(0x4)})
	goassert.DeepEquals(t, user.RoleNames(), ch.TimedSet{"square": ch.NewVbSimpleSequence(0x3), "nonexistent": ch.NewVbSimpleSequence(0x42), "frood": ch.NewVbSimpleSequence(0x4)})
	require.NoError(t, auth.Save(user))

	user2, err := auth.GetUser("arthur")
	assert.Equal(t, nil, err)
	log.Printf("Channels = %s", user2.Channels())
	goassert.DeepEquals(t, user2.Channels(), ch.AtSequence(ch.SetOf(t, "!", "britain"), 1))
	goassert.DeepEquals(t, user2.InheritedChannels(),
		ch.TimedSet{"!": ch.NewVbSimpleSequence(0x1), "britain": ch.NewVbSimpleSequence(0x1), "dull": ch.NewVbSimpleSequence(0x3), "duller": ch.NewVbSimpleSequence(0x3), "dullest": ch.NewVbSimpleSequence(0x3), "hoopy": ch.NewVbSimpleSequence(0x4), "hoopier": ch.NewVbSimpleSequence(0x4), "hoopiest": ch.NewVbSimpleSequence(0x4)})
	assert.True(t, user2.CanSeeChannel("britain"))
	assert.True(t, user2.CanSeeChannel("duller"))
	assert.True(t, user2.CanSeeChannel("hoopy"))
	assert.Equal(t, nil, user2.AuthorizeAllChannels(ch.SetOf(t, "britain", "dull", "hoopiest")))
}

func TestRegisterUser(t *testing.T) {
	gTestBucket := base.GetTestBucket(t)
	defer gTestBucket.Close()

	// Register user based on name, email
	auth := NewAuthenticator(gTestBucket.Bucket, nil)
	user, err := auth.RegisterNewUser("ValidName", "foo@example.com")
	require.NoError(t, err)
	assert.Equal(t, "ValidName", user.Name())
	assert.Equal(t, "foo@example.com", user.Email())

	// verify retrieval by username
	user, err = auth.GetUser("ValidName")
	require.NoError(t, err)
	assert.Equal(t, "ValidName", user.Name())

	// verify retrieval by email
	user, err = auth.GetUserByEmail("foo@example.com")
	require.NoError(t, err)
	assert.Equal(t, "ValidName", user.Name())

	// Register user based on email, retrieve based on username, email
	user, err = auth.RegisterNewUser("bar@example.com", "bar@example.com")
	require.NoError(t, err)
	assert.Equal(t, "bar@example.com", user.Name())
	assert.Equal(t, "bar@example.com", user.Email())

	user, err = auth.GetUser("UnknownName")
	require.NoError(t, err)
	assert.Equal(t, nil, user)

	user, err = auth.GetUserByEmail("bar@example.com")
	require.NoError(t, err)
	assert.Equal(t, "bar@example.com", user.Name())

	// Register user without an email address
	user, err = auth.RegisterNewUser("01234567890", "")
	require.NoError(t, err)
	assert.Equal(t, "01234567890", user.Name())
	assert.Equal(t, "", user.Email())
	// Get above user by username.
	user, err = auth.GetUser("01234567890")
	require.NoError(t, err)
	assert.Equal(t, "01234567890", user.Name())
	assert.Equal(t, "", user.Email())
	// Make sure we can't retrieve 01234567890 by supplying empty email.
	user, err = auth.GetUserByEmail("")
	require.NoError(t, err)
	assert.Equal(t, nil, user)

	// Try to register a user based on invalid email
	user, err = auth.RegisterNewUser("foo", "bar")
	require.NoError(t, err)
	assert.Equal(t, "foo", user.Name())
	assert.Equal(t, "", user.Email()) // skipped due to invalid email

}

func TestConcurrentUserWrites(t *testing.T) {
	gTestBucket := base.GetTestBucket(t)
	defer gTestBucket.Close()

	username := "foo"
	password := "password"
	email := "foo@bar.org"

	// Modify the bcrypt cost to test rehashPassword properly below
	SetBcryptCost(5)

	// Create user
	auth := NewAuthenticator(gTestBucket.Bucket, nil)
	user, _ := auth.NewUser(username, password, ch.SetOf(t, "123", "456"))
	user.SetExplicitRoles(ch.TimedSet{"role1": ch.NewVbSimpleSequence(1), "role2": ch.NewVbSimpleSequence(1)})
	createErr := auth.Save(user)
	if createErr != nil {
		t.Errorf("Error creating user: %v", createErr)
	}

	// Retrieve user to trigger initial calculation of roles, channels
	user, getErr := auth.GetUser(username)
	if getErr != nil {
		t.Errorf("Error retrieving user: %v", getErr)
	}

	SetBcryptCost(bcryptDefaultCost)
	// Reset bcryptCostChanged state after test runs
	defer func() {
		bcryptCostChanged = false
	}()

	var wg sync.WaitGroup
	wg.Add(4)
	// Update user email, password hash, and invalidate user channels, roles concurrently
	go func() {
		user, getErr := auth.GetUser(username)
		if getErr != nil {
			t.Errorf("Error retrieving user: %v", getErr)
		}
		if user == nil {
			t.Errorf("User is nil prior to invalidate channels, error: %v", getErr)
		}

		invalidateErr := auth.InvalidateChannels(user)
		if invalidateErr != nil {
			t.Errorf("Error invalidating user's channels: %v", invalidateErr)
		}
		wg.Done()
	}()

	go func() {
		user, getErr := auth.GetUser(username)
		if getErr != nil {
			t.Errorf("Error retrieving user: %v", getErr)
		}
		if user == nil {
			t.Errorf("User is nil prior to email update, error: %v", getErr)
		}

		updateErr := auth.UpdateUserEmail(user, email)
		if updateErr != nil {
			t.Errorf("Error updating user email: %v", updateErr)
		}
		wg.Done()
	}()

	go func() {
		user, getErr := auth.GetUser(username)
		if getErr != nil {
			t.Errorf("Error retrieving user: %v", getErr)
		}
		if user == nil {
			t.Errorf("User is nil prior to invalidate roles, error: %v", getErr)
		}

		updateErr := auth.InvalidateRoles(user)
		if updateErr != nil {
			t.Errorf("Error invalidating roles: %v", updateErr)
		}
		wg.Done()
	}()

	go func() {
		user, getErr := auth.GetUser(username)
		if getErr != nil {
			t.Errorf("Error retrieving user: %v", getErr)
		}
		if user == nil {
			t.Errorf("User is nil prior to invalidate roles, error: %v", getErr)
		}

		rehashErr := auth.rehashPassword(user, password)
		if rehashErr != nil {
			t.Errorf("Error rehashing password: %v", rehashErr)
		}
		wg.Done()
	}()

	wg.Wait()

	// Get the user, validate channels and email
	user, getErr = auth.GetUser(username)
	if getErr != nil {
		t.Errorf("Error retrieving user: %v", getErr)
	}

	assert.Equal(t, email, user.Email())
	assert.Equal(t, 3, len(user.Channels()))
	assert.Equal(t, 2, len(user.RoleNames()))

	// Check the password hash bcrypt cost
	userImpl := user.(*userImpl)
	cost, _ := bcrypt.Cost(userImpl.PasswordHash_)
	assert.Equal(t, cost, bcryptDefaultCost)
}
