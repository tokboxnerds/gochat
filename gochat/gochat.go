package gochat

import (
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
    Presence string
}

func bareJid(str string) string {
	return strings.Split(str, "/")[0]
}

func username(str string) string {
	return strings.Split(str, "@")[0]
}

func init() {
    http.HandleFunc("/_ah/xmpp/presence/available/", handleOnline)
    http.HandleFunc("/_ah/xmpp/presence/unavailable/", handleOffline)
    xmpp.Handle(handleChat)
}

func handleOnline(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	updatePresence(c, "online", bareJid(r.FormValue("from")))
}


func handleOffline(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	updatePresence(c, "offline", bareJid(r.FormValue("from")))
}

func updatePresence(c appengine.Context, status string, sender string) {
	q := datastore.NewQuery("User").Filter("JID =", sender).Limit(100)

	users := make([]User, 0, 100)
	keys, _ := q.GetAll(c, &users)

	i := 0
	for _, user := range users {
		user.Presence = status
		datastore.Put(c, keys[i], &user)
		i++;
	}
}

func handleHelp(c appengine.Context, m *xmpp.Message) {
	reply := &xmpp.Message{
	    Sender: m.To[0],
	    To: []string{m.Sender},
	    Body: "/meet Create a meet tokbox room\r\n/list Show the list of people subscribed to this room\r\n/pp Time for ping pong" ,
	}
	reply.Send(c)
}

func handleList(c appengine.Context, m *xmpp.Message) {
	room := username(m.To[0])
	q := datastore.NewQuery("User").Filter("Room =", room).Limit(100)
	users := make([]User, 0, 100)
	q.GetAll(c, &users)

	var names []string
	for _, user := range users {
		name := user.Name
		if name == "" {
			name = username(user.JID)
		}

		if user.Presence == "online" {
			name += " [" + user.Presence + "]"
		}
		names = append(names, name)
	}

	reply := &xmpp.Message{
	    Sender: m.To[0],
	    To: []string{m.Sender},
	    Body: "People in this room: " + strings.Join(names, ", "),
	}
	reply.Send(c)
}

func broadcast(c appengine.Context, m *xmpp.Message, body string) {
	room := username(m.To[0])
	sender := bareJid(m.Sender)
	name := username(sender)

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
	room := ""
	fields := strings.Fields(m.Body)
	if len(fields) > 1 {
		room = fields[1]
	} else {
		room = username(m.To[0])
	}

	broadcast(c, m, "Connect to this room https://meet.tokbox.com/" + room)
}

func handlePingPong(c appengine.Context, m *xmpp.Message) {
	broadcast(c, m, "Time for ping pong")
}

var commands = map[string]func(appengine.Context, *xmpp.Message)() {
	"help": handleHelp,
	"list": handleList,
	"meet": handleMeet,
	"pp": handlePingPong,
}

func handleChat(c appengine.Context, m *xmpp.Message) {
	room := username(m.To[0])
	sender := bareJid(m.Sender)
	name := username(sender)

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
	        // nickname extension only supported in presence stanzas by PSI :(
	        //Body: "<body>" + m.Body + "</body><nick xmlns='http://jabber.org/protocol/nick'>" + strings.Split(m.Sender, "@")[0] + "</nick>",
	        Body: "[" + name + "] " + m.Body,
	        //RawXML: true,
	    }
	    reply.Send(c)
	}

	if found && command {
		key := strings.Fields(m.Body)[0][1:]
		commands[key](c, m)
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
	        Presence: "online",
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