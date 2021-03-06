package models

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"cloud.google.com/go/datastore"

	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/errors"
)

func swapKeys(keyList []*datastore.Key, oldKey, newKey *datastore.Key) []*datastore.Key {
	for i := range keyList {
		if keyList[i].Equal(oldKey) {
			keyList[i] = newKey
		}
	}

	// Remove duplicates
	var clean []*datastore.Key
	seen := map[string]struct{}{}
	for i := range keyList {
		keyString := keyList[i].String()
		if _, hasVal := seen[keyString]; !hasVal {
			seen[keyString] = struct{}{}
			clean = append(clean, keyList[i])
		}
	}

	return clean
}

func swapReadUserKeys(readList []*Read, oldKey, newKey *datastore.Key) []*Read {
	var clean []*Read
	seen := map[string]struct{}{}
	for i := range readList {
		keyString := readList[i].UserKey.String()
		if _, isSeen := seen[keyString]; !isSeen {
			seen[keyString] = struct{}{}

			if readList[i].UserKey.Equal(oldKey) {
				readList[i].UserKey = newKey
			}

			clean = append(clean, readList[i])
		}
	}

	return clean
}

func mergeContacts(a, b []*datastore.Key) []*datastore.Key {
	var all []*datastore.Key
	all = append(all, a...)
	all = append(all, b...)

	var merged []*datastore.Key
	seen := make(map[string]struct{})

	for i := range all {
		keyString := all[i].String()

		if _, isSeen := seen[keyString]; isSeen {
			continue
		}

		seen[keyString] = struct{}{}
		merged = append(merged, all[i])
	}

	return merged
}

func reassignContacts(ctx context.Context, tx *datastore.Transaction, oldUser, newUser *User) error {
	var users []*User
	q := datastore.NewQuery("User").Filter("ContactKeys =", oldUser.Key)
	keys, err := db.DefaultClient.GetAll(ctx, q, &users)
	if err != nil {
		return err
	}

	for i := range users {
		users[i].ContactKeys = swapKeys(users[i].ContactKeys, oldUser.Key, newUser.Key)
	}

	_, err = tx.PutMulti(keys, users)
	if err != nil {
		return err
	}

	return nil
}

func reassignMessageUsers(ctx context.Context, tx *datastore.Transaction, old, newUser *User) error {
	userMessages, err := GetUnhydratedMessagesByUser(ctx, old)
	if err != nil {
		return err
	}

	// Reassign ownership of messages and save keys to oldUserMessageKeys slice
	userMessageKeys := make([]*datastore.Key, len(userMessages))
	for i := range userMessages {
		userMessages[i].UserKey = newUser.Key
		userMessages[i].Reads = swapReadUserKeys(userMessages[i].Reads, old.Key, newUser.Key)
		userMessageKeys[i] = userMessages[i].Key
	}

	// Save the messages
	_, err = tx.PutMulti(userMessageKeys, userMessages)
	if err != nil {
		return err
	}

	return nil
}

func reassignThreadUsers(ctx context.Context, tx *datastore.Transaction, old, newUser *User) error {
	userThreads, err := GetUnhydratedThreadsByUser(ctx, old, &Pagination{Size: -1})
	if err != nil {
		return err
	}

	// Reassign ownership of threads and save keys to oldUserThreadKeys slice
	userThreadKeys := make([]*datastore.Key, len(userThreads))
	for i := range userThreads {
		userThreads[i].UserKeys = swapKeys(userThreads[i].UserKeys, old.Key, newUser.Key)
		userThreads[i].Reads = swapReadUserKeys(userThreads[i].Reads, old.Key, newUser.Key)

		if userThreads[i].OwnerKey.Equal(old.Key) {
			userThreads[i].OwnerKey = newUser.Key
		}

		userThreadKeys[i] = userThreads[i].Key
	}

	// Save the threads
	_, err = tx.PutMulti(userThreadKeys, userThreads)
	if err != nil {
		return err
	}

	return nil
}

func reassignEventUsers(ctx context.Context, tx *datastore.Transaction, old, newUser *User) error {
	userEvents, err := GetUnhydratedEventsByUser(ctx, old, &Pagination{Size: -1})
	if err != nil {
		return err
	}

	// Reassign ownership of events and save keys to userEvetKeys slice
	userEventKeys := make([]*datastore.Key, len(userEvents))
	for i := range userEvents {
		userEvents[i].UserKeys = swapKeys(userEvents[i].UserKeys, old.Key, newUser.Key)
		userEvents[i].RSVPKeys = swapKeys(userEvents[i].RSVPKeys, old.Key, newUser.Key)
		userEvents[i].Reads = swapReadUserKeys(userEvents[i].Reads, old.Key, newUser.Key)

		if userEvents[i].OwnerKey.Equal(old.Key) {
			userEvents[i].OwnerKey = newUser.Key
		}

		userEventKeys[i] = userEvents[i].Key
	}

	// Save the events
	_, err = tx.PutMulti(userEventKeys, userEvents)
	if err != nil {
		return err
	}

	return nil
}

func readStringFromFile(file string) string {
	op := errors.Opf("models.readStringFromFile(file=%s)", file)

	wd, err := os.Getwd()
	if err != nil {
		// This function should only be run at startup time, so we
		// panic if it fails.
		panic(errors.E(op, err))
	}

	var basePath string
	if strings.HasSuffix(wd, "models") || strings.HasSuffix(wd, "integ") {
		// This package is the cwd, so we need to go up one dir to resolve the
		// layouts and includes dirs consistently.
		basePath = "../models/content"
	} else {
		basePath = "./models/content"
	}

	b, err := ioutil.ReadFile(path.Join(basePath, file))
	if err != nil {
		panic(err)
	}

	return string(b)
}
