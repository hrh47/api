package models

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/arran4/golang-ical"
	"github.com/gosimple/slug"

	"github.com/hiconvo/api/db"
)

type Event struct {
	Key          *datastore.Key   `json:"-"        datastore:"__key__"`
	ID           string           `json:"id"       datastore:"-"`
	OwnerKey     *datastore.Key   `json:"-"`
	Owner        *UserPartial     `json:"owner"    datastore:"-"`
	UserKeys     []*datastore.Key `json:"-"`
	UserPartials []*UserPartial   `json:"users"    datastore:"-"`
	Users        []*User          `json:"-"        datastore:"-"`
	RSVPKeys     []*datastore.Key `json:"-"`
	RSVPs        []*UserPartial   `json:"rsvps"    datastore:"-"`
	PlaceID      string           `json:"placeID"  datastore:",noindex"`
	Address      string           `json:"address"  datastore:",noindex"`
	Lat          float64          `json:"lat"      datastore:",noindex"`
	Lng          float64          `json:"lng"      datastore:",noindex"`
	Name         string           `json:"name"     datastore:",noindex"`
	Description  string           `json:"description"  datastore:",noindex"`
	Timestamp    time.Time        `json:"timestamp"    datastore:",noindex"`
	UTCOffset    int              `json:"-"        datastore:",noindex"`
	UserReads    []*UserPartial   `json:"reads"    datastore:"-"`
	Reads        []*Read          `json:"-"        datastore:",noindex"`
	CreatedAt    time.Time        `json:"-"`
}

func NewEvent(
	name, description, placeID, address string,
	lat, lng float64,
	timestamp time.Time,
	utcOffset int,
	owner *User,
	users []*User,
) (Event, error) {
	// Get all of the users' keys, remove duplicates, and check whether
	// the owner was included in the users slice
	userKeys := make([]*datastore.Key, 0)
	seen := make(map[string]struct{})
	hasOwner := false
	for _, u := range users {
		if _, alreadySeen := seen[u.ID]; alreadySeen {
			continue
		}
		seen[u.ID] = struct{}{}
		if u.Key.Equal(owner.Key) {
			hasOwner = true
		}
		userKeys = append(userKeys, u.Key)
	}

	// Add the owner to the users if not already present
	if !hasOwner {
		userKeys = append(userKeys, owner.Key)
		users = append(users, owner)
	}

	return Event{
		Key:          datastore.IncompleteKey("Event", nil),
		OwnerKey:     owner.Key,
		Owner:        MapUserToUserPartial(owner),
		UserKeys:     userKeys,
		UserPartials: MapUsersToUserPartials(users),
		Users:        users,
		Name:         name,
		PlaceID:      placeID,
		Address:      address,
		Lat:          lat,
		Lng:          lng,
		Timestamp:    timestamp,
		UTCOffset:    utcOffset,
		Description:  description,
	}, nil
}

func (e *Event) LoadKey(k *datastore.Key) error {
	e.Key = k

	// Add URL safe key
	e.ID = k.Encode()

	return nil
}

func (e *Event) Save() ([]datastore.Property, error) {
	return datastore.SaveStruct(e)
}

func (e *Event) Load(ps []datastore.Property) error {
	if err := datastore.LoadStruct(e, ps); err != nil {
		return err
	}

	return nil
}

func (e *Event) Commit(ctx context.Context) error {
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}

	key, kErr := db.Client.Put(ctx, e.Key, e)
	if kErr != nil {
		return kErr
	}
	e.ID = key.Encode()
	e.Key = key
	return nil
}

func (e *Event) Delete(ctx context.Context) error {
	if err := db.Client.Delete(ctx, e.Key); err != nil {
		return err
	}
	return nil
}

func (e *Event) GetReads() []*Read {
	return e.Reads
}

func (e *Event) SetReads(newReads []*Read) {
	e.Reads = newReads
}

func (e *Event) GetKey() *datastore.Key {
	return e.Key
}

func (e *Event) GetName() string {
	return e.Name
}

func (e *Event) GetFormatedTime() string {
	loc := time.FixedZone("Given", e.UTCOffset)
	return e.Timestamp.In(loc).Format("Monday, January 2 @ 3:04 PM")
}

func (e *Event) HasUser(u *User) bool {
	for _, k := range e.UserKeys {
		if k.Equal(u.Key) {
			return true
		}
	}

	return false
}

func (e *Event) OwnerIs(u *User) bool {
	return e.OwnerKey.Equal(u.Key)
}

func (e *Event) HasRSVP(u *User) bool {
	for _, k := range e.RSVPKeys {
		if k.Equal(u.Key) {
			return true
		}
	}

	return false
}

// AddUser adds a user to the event.
func (e *Event) AddUser(u *User) error {
	// Cannot add owner or duplicate.
	if e.OwnerIs(u) || e.HasUser(u) {
		return errors.New("This user is already invited to this event")
	}

	if len(e.UserKeys) >= 300 {
		return errors.New("This event has the maximum number of guests")
	}

	e.UserKeys = append(e.UserKeys, u.Key)
	e.UserPartials = append(e.UserPartials, MapUserToUserPartial(u))

	return nil
}

func (e *Event) RemoveUser(u *User) {
	// Remove from keys.
	for i, k := range e.UserKeys {
		if k.Equal(u.Key) {
			e.UserKeys[i] = e.UserKeys[len(e.UserKeys)-1]
			e.UserKeys = e.UserKeys[:len(e.UserKeys)-1]
			break
		}
	}
	// Remove from partials.
	for i, c := range e.UserPartials {
		if c.ID == u.ID {
			e.UserPartials[i] = e.UserPartials[len(e.UserPartials)-1]
			e.UserPartials = e.UserPartials[:len(e.UserPartials)-1]
			break
		}
	}
}

// AddRSVP RSVPs a user for the event.
func (e *Event) AddRSVP(u *User) error {
	// Cannot add owner or duplicate.
	if e.OwnerIs(u) || e.HasRSVP(u) {
		return errors.New("This user has already RSVP'd")
	}

	e.RSVPKeys = append(e.RSVPKeys, u.Key)
	e.RSVPs = append(e.RSVPs, MapUserToUserPartial(u))
	e.SetReads([]*Read{})

	return nil
}

func (e *Event) RemoveRSVP(u *User) {
	// Remove from keys.
	for i, k := range e.RSVPKeys {
		if k.Equal(u.Key) {
			e.RSVPKeys[i] = e.RSVPKeys[len(e.RSVPKeys)-1]
			e.RSVPKeys = e.RSVPKeys[:len(e.RSVPKeys)-1]
			break
		}
	}

	// Remove from partials.
	for i, c := range e.RSVPs {
		if c.ID == u.ID {
			e.RSVPs[i] = e.RSVPs[len(e.RSVPs)-1]
			e.RSVPs = e.RSVPs[:len(e.RSVPs)-1]
			break
		}
	}
}

func (e *Event) GetEmail() string {
	slugified := slug.Make(e.Name)
	if len(slugified) > 20 {
		slugified = slugified[:20]
	}
	return fmt.Sprintf("%s-%d@mail.hiconvo.com", slugified, e.Key.ID)
}

func (e *Event) SendInvites(ctx context.Context) error {
	return sendEvent(e, false)
}

func (e *Event) SendUpdatedInvites(ctx context.Context) error {
	return sendEvent(e, true)
}

func (e *Event) SendInviteToUser(ctx context.Context, user *User) error {
	return sendEventInvitation(e, user)
}

func (e *Event) SendCancellation(ctx context.Context, message string) error {
	return sendCancellation(e, message)
}

func (e *Event) IsInFuture() bool {
	return e.Timestamp.After(time.Now())
}

func (e *Event) IsUpcoming() bool {
	start, err := time.ParseDuration("6h")
	if err != nil {
		return false
	}
	end, err := time.ParseDuration("30h")
	if err != nil {
		return false
	}

	windowStart := time.Now().Add(start)
	windowEnd := time.Now().Add(end)

	return e.Timestamp.After(windowStart) && e.Timestamp.Before(windowEnd)
}

func (e *Event) GetICS() string {
	cal := ics.NewCalendar()

	ev := cal.AddEvent(e.ID)

	ev.SetCreatedTime(e.CreatedAt)
	ev.SetStartAt(e.Timestamp)
	ev.SetEndAt(e.Timestamp.Add(time.Hour))
	ev.SetSummary(e.Name)
	ev.SetLocation(e.Address)
	ev.SetDescription(e.Description)
	ev.SetOrganizer(e.GetEmail(), ics.WithCN(e.Owner.FullName))

	return cal.Serialize()
}

func GetEventByID(ctx context.Context, id string) (Event, error) {
	var e Event

	key, err := datastore.DecodeKey(id)
	if err != nil {
		return e, err
	}

	return handleGetEvent(ctx, key, e)
}

func GetUnhydratedEventsByUser(ctx context.Context, u *User) ([]*Event, error) {
	var events []*Event
	q := datastore.NewQuery("Event").Filter("UserKeys =", u.Key)
	_, err := db.Client.GetAll(ctx, q, &events)
	if err != nil {
		return events, err
	}

	return events, nil
}

func GetEventsByUser(ctx context.Context, u *User) ([]*Event, error) {
	// Get all of the events of which the user is a member
	events, err := GetUnhydratedEventsByUser(ctx, u)
	if err != nil {
		return events, err
	}

	// Now that we have the events, we need to get the users. We keep track of
	// where the users of one event start and another begin by incrementing
	// an index.
	var uKeys []*datastore.Key
	var idxs []int
	for _, e := range events {
		uKeys = append(uKeys, e.UserKeys...)
		idxs = append(idxs, len(e.UserKeys))
	}

	// We get all of the users in one go.
	userPtrs := make([]*User, len(uKeys))
	if err := db.Client.GetMulti(ctx, uKeys, userPtrs); err != nil {
		return events, err
	}

	// We add the just retrieved user objects to their corresponding events by
	// iterating through all of the events and assigning their users according
	// to the index which we created above.
	//
	// We also create a new slice of pointers to events which we'll finally
	// return.
	start := 0
	eventPtrs := make([]*Event, len(events))
	for i := range events {
		eventUsers := userPtrs[start : start+idxs[i]]

		eventRSVPs := make([]*User, 0)
		var owner *User
		for j := range eventUsers {

			if events[i].HasRSVP(eventUsers[j]) {
				eventRSVPs = append(eventRSVPs, eventUsers[j])
			}

			if events[i].OwnerIs(eventUsers[j]) {
				owner = eventUsers[j]
			}
		}

		events[i].Owner = MapUserToUserPartial(owner)
		events[i].UserPartials = MapUsersToUserPartials(eventUsers)
		events[i].Users = eventUsers
		events[i].RSVPs = MapUsersToUserPartials(eventRSVPs)
		events[i].UserReads = MapReadsToUserPartials(events[i], eventUsers)

		start += idxs[i]
		eventPtrs[i] = events[i]
	}

	return eventPtrs, nil
}

func handleGetEvent(ctx context.Context, key *datastore.Key, e Event) (Event, error) {
	if err := db.Client.Get(ctx, key, &e); err != nil {
		return e, err
	}

	users := make([]User, len(e.UserKeys))
	if err := db.Client.GetMulti(ctx, e.UserKeys, users); err != nil {
		return e, err
	}

	userPointers := make([]*User, len(users))
	rsvpPointers := make([]*User, 0)
	var owner User
	for i := range users {
		userPointers[i] = &users[i]

		if e.OwnerKey.Equal(users[i].Key) {
			owner = users[i]
		}

		if e.HasRSVP(userPointers[i]) {
			rsvpPointers = append(rsvpPointers, userPointers[i])
		}
	}

	e.UserPartials = MapUsersToUserPartials(userPointers)
	e.Users = userPointers
	e.Owner = MapUserToUserPartial(&owner)
	e.RSVPs = MapUsersToUserPartials(rsvpPointers)
	e.UserReads = MapReadsToUserPartials(&e, userPointers)

	return e, nil
}
