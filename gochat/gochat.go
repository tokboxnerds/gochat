package gochat

import (
    "fmt"
    "net/http"
    "time"
    "strings"
    "appengine"
    "appengine/datastore"
    "appengine/xmpp"
)

type User struct {
    Room  string
    JID   string
    Date  time.Time
}

func init() {
    http.HandleFunc("/", handler)
    xmpp.Handle(handleChat)
}

func handler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprint(w, "Hello, go chat!")
}

func handleChat(c appengine.Context, m *xmpp.Message) {
	room := strings.Split(m.To[0], "@")[0]
	sender := strings.Split(m.Sender, "/")[0]

	q := datastore.NewQuery("User").Filter("Room =", room).Limit(100)
	users := make([]User, 0, 100)
	q.GetAll(c, &users);

	found := false
	for _, user := range users {
		if user.JID == sender {
	    	found = true
	    	continue
	    }

		reply := &xmpp.Message{
	    	Sender: m.To[0],
	        To: []string{user.JID},
	        Body: "[" + strings.Split(m.Sender, "@")[0] + "] " + m.Body,
	    }
	    reply.Send(c)
	}
	if !found {
		key := datastore.NewIncompleteKey(c, "User", nil)
		u := &User{
	        Room: room,
	        JID: sender,
	        Date: time.Now(),
	    }
	    _, err := datastore.Put(c, key, u)
	    if err != nil {
	    	c.Errorf("Sending reply: %v", err)
	    }
	   	reply := &xmpp.Message{
	    	Sender: m.To[0],
	        To: []string{u.JID},
	        Body: "Welcome to the room " + room + "!!!",
	    }
	    reply.Send(c)
	}
}
