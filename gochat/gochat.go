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
    Name  string
    Date  time.Time
}

func init() {
    http.HandleFunc("/", handler)
    xmpp.Handle(handleChat)
}

func handler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprint(w, "Hello, go chat!")
}

func handleHelp(c appengine.Context, m *xmpp.Message) {
	reply := &xmpp.Message{
	    Sender: m.To[0],
	    To: []string{m.Sender},
	    Body: "/meet Create a meet tokbox room\r\n/list Show the list of people subscribed to this room" ,
	}
	reply.Send(c)
}

func handleList(c appengine.Context, m *xmpp.Message) {
	room := strings.Split(m.To[0], "@")[0]
	q := datastore.NewQuery("User").Filter("Room =", room).Limit(100)
	users := make([]User, 0, 100)
	q.GetAll(c, &users)

	var names []string
	for _, user := range users {
		name := user.Name
		if name == "" {
			name = strings.Split(user.JID, "@")[0]
		}
		names = append(names, name)
	}

	reply := &xmpp.Message{
	    Sender: m.To[0],
	    To: []string{m.Sender},
	    Body: "People in this room " + strings.Join(names, ", "),
	}
	reply.Send(c)
}

func broadcast(c appengine.Context, m *xmpp.Message, body string) {
	room := strings.Split(m.To[0], "@")[0]
	sender := strings.Split(m.Sender, "/")[0]
	name := strings.Split(sender, "@")[0]

	q := datastore.NewQuery("User").Filter("Room =", room).Limit(100)
	users := make([]User, 0, 100)
	q.GetAll(c, &users)

	for _, user := range users {
		reply := &xmpp.Message{
	    	Sender: m.To[0],
	        To: []string{user.JID},
	        Body: "[" + name + "] " + body,
	    }
	    reply.Send(c)
	}
}

func handleMeet(c appengine.Context, m *xmpp.Message) {
	room := strings.Split(m.To[0], "@")[0]

	broadcast(c, m, "Connect to this room https://meet.tokbox.com/" + room)
}

var commands = map[string]func(appengine.Context, *xmpp.Message)() {
	"help": handleHelp,
	"list": handleList,
	"meet": handleMeet,
}

func handleChat(c appengine.Context, m *xmpp.Message) {
	room := strings.Split(m.To[0], "@")[0]
	sender := strings.Split(m.Sender, "/")[0]
	name := strings.Split(sender, "@")[0]

	q := datastore.NewQuery("User").Filter("Room =", room).Limit(100)
	users := make([]User, 0, 100)
	q.GetAll(c, &users)

	command := strings.HasPrefix(m.Body, "/")

	found := false
	for _, user := range users {
		if user.JID == sender {
	    	found = true
	    	continue
	    }

	    if command {
	    	continue
	    }

		reply := &xmpp.Message{
	    	Sender: m.To[0],
	        To: []string{user.JID},
	        //Body: "<message to='" + sender + "' type='chat'><body>" + m.Body + "</body><nick xmlns='http://jabber.org/protocol/nick'>" + strings.Split(m.Sender, "@")[0] + "</nick></message>",
	        Body: "[" + name + "] " + m.Body,
	        //RawXML: true,
	    }
	    reply.Send(c)
	}

	if found && command {
		commands[m.Body[1:]](c, m)
	}

	if !found {
		if strings.Split(sender, "@")[1] != "tokbox.com" {
			reply := &xmpp.Message{
	    		Sender: m.To[0],
	        	To: []string{sender},
	        	Body: "You are not authorized to join this room",
	    	}
	    	reply.Send(c)
			return
		}

		key := datastore.NewIncompleteKey(c, "User", nil)
		u := &User{
	        Room: room,
	        JID: sender,
	        Name: name,
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
