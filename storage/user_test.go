package storage_test

import (
	"io/ioutil"
	"os"
	"strconv"
	"testing"

	"github.com/khlieng/dispatch/storage"
	"github.com/khlieng/dispatch/storage/bleve"
	"github.com/khlieng/dispatch/storage/boltdb"
	"github.com/kjk/betterguid"
	"github.com/stretchr/testify/assert"
)

func tempdir() string {
	f, _ := ioutil.TempDir("", "")
	return f
}

func TestUser(t *testing.T) {
	storage.Initialize(tempdir(), "", "")

	db, err := boltdb.New(storage.Path.Database())
	assert.Nil(t, err)

	storage.GetMessageStore = func(_ *storage.User) (storage.MessageStore, error) {
		return db, nil
	}
	storage.GetMessageSearchProvider = func(_ *storage.User) (storage.MessageSearchProvider, error) {
		return nil, nil
	}

	user, err := storage.NewUser(db)
	assert.Nil(t, err)

	srv := &storage.Network{
		Name: "freenode",
		Host: "irc.freenode.net",
		Nick: "test",
	}
	chan1 := &storage.Channel{
		Network: srv.Host,
		Name:    "#test",
	}
	chan2 := &storage.Channel{
		Network: srv.Host,
		Name:    "#testing",
	}

	user.SaveNetwork(srv)
	user.SaveChannel(chan1)
	user.SaveChannel(chan2)

	users, err := storage.LoadUsers(db)
	assert.Nil(t, err)
	assert.Len(t, users, 1)

	user = users[0]
	assert.Equal(t, uint64(1), user.ID)

	servers, err := user.Networks()
	assert.Len(t, servers, 1)
	assert.Equal(t, srv, servers[0])

	channels, err := user.Channels()
	assert.Len(t, channels, 2)
	assert.Equal(t, chan1, channels[0])
	assert.Equal(t, chan2, channels[1])

	user.SetNick("bob", srv.Host)
	servers, err = user.Networks()
	assert.Equal(t, "bob", servers[0].Nick)

	user.SetNetworkName("cake", srv.Host)
	servers, err = user.Networks()
	assert.Equal(t, "cake", servers[0].Name)

	user.RemoveChannel(srv.Host, chan1.Name)
	channels, err = user.Channels()
	assert.Len(t, channels, 1)
	assert.Equal(t, chan2, channels[0])

	user.RemoveNetwork(srv.Host)
	servers, err = user.Networks()
	assert.Len(t, servers, 0)
	channels, err = user.Channels()
	assert.Len(t, channels, 0)

	user.AddOpenDM(srv.Host, "cake")
	openDMs, err := user.OpenDMs()
	assert.Nil(t, err)
	assert.Len(t, openDMs, 1)
	err = user.RemoveOpenDM(srv.Host, "cake")
	assert.Nil(t, err)
	openDMs, err = user.OpenDMs()
	assert.Nil(t, err)
	assert.Len(t, openDMs, 0)

	settings := user.ClientSettings()
	assert.NotNil(t, settings)
	assert.Equal(t, storage.DefaultClientSettings(), settings)

	settings.ColoredNicks = !settings.ColoredNicks
	err = user.SetClientSettings(settings)
	assert.Nil(t, err)
	assert.Equal(t, settings, user.ClientSettings())
	assert.NotEqual(t, settings, storage.DefaultClientSettings())

	user.AddOpenDM(srv.Host, "cake")

	user.Remove()
	_, err = os.Stat(storage.Path.User(user.Username))
	assert.True(t, os.IsNotExist(err))

	openDMs, err = user.OpenDMs()
	assert.Nil(t, err)
	assert.Len(t, openDMs, 0)

	users, err = storage.LoadUsers(db)
	assert.Nil(t, err)

	for i := range users {
		assert.NotEqual(t, user.ID, users[i].ID)
	}
}

func TestMessages(t *testing.T) {
	storage.Initialize(tempdir(), "", "")

	db, err := boltdb.New(storage.Path.Database())
	assert.Nil(t, err)

	storage.GetMessageStore = func(_ *storage.User) (storage.MessageStore, error) {
		return db, nil
	}
	storage.GetMessageSearchProvider = func(user *storage.User) (storage.MessageSearchProvider, error) {
		return bleve.New(storage.Path.Index(user.Username))
	}

	user, err := storage.NewUser(db)
	assert.Nil(t, err)

	os.MkdirAll(storage.Path.User(user.Username), 0700)

	messages, hasMore, err := user.Messages("irc.freenode.net", "#go-nuts", 10, "6")
	assert.Nil(t, err)
	assert.False(t, hasMore)
	assert.Len(t, messages, 0)

	messages, hasMore, err = user.LastMessages("irc.freenode.net", "#go-nuts", 10)
	assert.Nil(t, err)
	assert.False(t, hasMore)
	assert.Len(t, messages, 0)

	messages, err = user.SearchMessages("irc.freenode.net", "#go-nuts", "message")
	assert.Nil(t, err)
	assert.Len(t, messages, 0)

	ids := []string{}
	for i := 0; i < 5; i++ {
		id := betterguid.New()
		ids = append(ids, id)
		err = user.LogMessage(&storage.Message{
			ID:      id,
			Network: "irc.freenode.net",
			From:    "nick",
			To:      "#go-nuts",
			Content: "message" + strconv.Itoa(i),
		})
		assert.Nil(t, err)
	}

	messages, hasMore, err = user.Messages("irc.freenode.net", "#go-nuts", 10, ids[4])
	assert.Equal(t, "message0", messages[0].Content)
	assert.Equal(t, "message3", messages[3].Content)
	assert.Nil(t, err)
	assert.False(t, hasMore)
	assert.Len(t, messages, 4)

	messages, hasMore, err = user.Messages("irc.freenode.net", "#go-nuts", 10, betterguid.New())
	assert.Equal(t, "message0", messages[0].Content)
	assert.Equal(t, "message4", messages[4].Content)
	assert.Nil(t, err)
	assert.False(t, hasMore)
	assert.Len(t, messages, 5)

	messages, hasMore, err = user.Messages("irc.freenode.net", "#go-nuts", 10, ids[2])
	assert.Equal(t, "message0", messages[0].Content)
	assert.Equal(t, "message1", messages[1].Content)
	assert.Nil(t, err)
	assert.False(t, hasMore)
	assert.Len(t, messages, 2)

	messages, hasMore, err = user.LastMessages("irc.freenode.net", "#go-nuts", 10)
	assert.Equal(t, "message0", messages[0].Content)
	assert.Equal(t, "message4", messages[4].Content)
	assert.Nil(t, err)
	assert.False(t, hasMore)
	assert.Len(t, messages, 5)

	messages, hasMore, err = user.LastMessages("irc.freenode.net", "#go-nuts", 4)
	assert.Equal(t, "message1", messages[0].Content)
	assert.Equal(t, "message4", messages[3].Content)
	assert.Nil(t, err)
	assert.True(t, hasMore)
	assert.Len(t, messages, 4)

	messages, err = user.SearchMessages("irc.freenode.net", "#go-nuts", "message")
	assert.Nil(t, err)
	assert.True(t, len(messages) > 0)

	user.LogEvent("irc.freenode.net", "join", []string{"bob"}, "#go-nuts")
	messages, hasMore, err = user.LastMessages("irc.freenode.net", "#go-nuts", 1)
	assert.Zero(t, messages[0].Content)
	assert.Nil(t, err)
	assert.True(t, hasMore)
	assert.Len(t, messages[0].Events, 1)
	assert.Equal(t, "join", messages[0].Events[0].Type)
	assert.NotZero(t, messages[0].Events[0].Time)

	user.LogEvent("irc.freenode.net", "part", []string{"bob"}, "#go-nuts")
	messages, hasMore, err = user.LastMessages("irc.freenode.net", "#go-nuts", 1)
	assert.Zero(t, messages[0].Content)
	assert.Nil(t, err)
	assert.True(t, hasMore)
	assert.Len(t, messages[0].Events, 2)
	assert.Equal(t, "part", messages[0].Events[1].Type)
	assert.NotZero(t, messages[0].Events[1].Time)

	user.LogEvent("irc.freenode.net", "nick", []string{"bob", "rob"}, "#go-nuts")
	messages, hasMore, err = user.LastMessages("irc.freenode.net", "#go-nuts", 1)
	assert.Zero(t, messages[0].Content)
	assert.Nil(t, err)
	assert.True(t, hasMore)
	assert.Len(t, messages[0].Events, 3)
	assert.Equal(t, "nick", messages[0].Events[2].Type)
	assert.NotZero(t, messages[0].Events[2].Time)

	user.LogEvent("irc.freenode.net", "quit", []string{"rob", "bored"}, "#go-nuts")
	messages, hasMore, err = user.LastMessages("irc.freenode.net", "#go-nuts", 1)
	assert.Zero(t, messages[0].Content)
	assert.Nil(t, err)
	assert.True(t, hasMore)
	assert.Len(t, messages[0].Events, 4)
	assert.Equal(t, "quit", messages[0].Events[3].Type)
	assert.Equal(t, []string{"rob", "bored"}, messages[0].Events[3].Params)
	assert.NotZero(t, messages[0].Events[3].Time)

	db.Close()
}
