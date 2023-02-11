package main

import "testing"

func TestUsersOnline(t *testing.T) {
	app := solution{
		UsersOnline: make(map[string]*onlineData),
	}

	pubkey := "test"
	app.markUserOnline(pubkey)

	if !app.isUserInOnlineData(pubkey) {
		t.Fatal("user should be online")
	}

	app.markUserOffline(pubkey)
	if app.isUserInOnlineData(pubkey) {
		t.Fatal("user should be offline")
	}
}
