package gochat

import (
    "net/http"
    "time"
    "fmt"
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

type Keyword struct {
    Room  string
    Value string
    LastUsed time.Time
    LastUser string
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
        Body: "/meet Create a meet tokbox room\r\n" +
        "/list Show the list of people subscribed to this room\r\n" +
        "/leave Leave this room\r\n" +
        "/watch [keyword] Keep track of a keyword\r\n" +
        "/unwatch [keyword] Stop tracking a keyword\r\n" +
        "/watchlist Keywords tracked in this room\r\n" +
        "/keyword [word] Info on a tracked keyword\r\n" +
        "/pp Time for ping pong" ,
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
    sender := bareJid(m.Sender)
    name := username(sender)
    announce(c, m, body, name)
}

func announce(c appengine.Context, m *xmpp.Message, body string, announcer string) {
    room := username(m.To[0])

    q := datastore.NewQuery("User").Filter("Room =", room).Limit(100)
    users := make([]User, 0, 100)
    q.GetAll(c, &users)

    for _, user := range users {
        reply := &xmpp.Message{
            Sender: m.To[0],
            To: []string{user.JID},
            Body: "[" + announcer + "] " + body,
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

func handleLeave(c appengine.Context, m *xmpp.Message) {
    room := username(m.To[0])
    sender := bareJid(m.Sender)
    name := username(sender)

    q := datastore.NewQuery("User").Filter("JID =", sender).Filter("Room =", room).Limit(1)

    users := make([]User, 0, 1)
    keys, _ := q.GetAll(c, &users)

    if len(keys) != 0 {
        datastore.Delete(c, keys[0])

        reply := &xmpp.Message{
            Sender: m.To[0],
            To: []string{m.Sender},
            Body: "Good bye. Come back soon!!!",
        }
        reply.Send(c)
    }
    //Send message to room
    broadcast(c, m, "User " + name + " has left the room")
}

func handlePingPong(c appengine.Context, m *xmpp.Message) {
    broadcast(c, m, "Time for ping pong")
}

func arg(arglist string, argnum int) string {
    return strings.Split(arglist, " ")[argnum]
}

func watchingKeyword(c appengine.Context, keyword string, room string) bool {
    q := datastore.NewQuery("Keyword").Filter("Value =", keyword).Filter("Room =", room).Limit(1)
    
    count, _ := q.Count(c)
    return count > 0
}

func handleWatch(c appengine.Context, m *xmpp.Message) {
    key := datastore.NewIncompleteKey(c, "Keyword", nil)
    keyword := strings.ToLower(arg(m.Body, 1))
	room := username(m.To[0])

    if (watchingKeyword(c, keyword, room)) {
        reply := &xmpp.Message{
            Sender: m.To[0],
            To: []string{m.Sender},
            Body: "Cannot watch watched keyword " + keyword,
        }
        reply.Send(c)
        return
    }

    entry := &Keyword{
        Room: room,
        Value: keyword,
        LastUsed: time.Now(),
    }
    _, err := datastore.Put(c, key, entry)
    if err != nil {
        c.Errorf("Storing Keyword: %v", err)
    }
    
    reply := &xmpp.Message{
        Sender: m.To[0],
        To: []string{m.Sender},
        Body: fmt.Sprintf("%v: Watching keyword %v", room, keyword),
    }
    reply.Send(c)
}

func handleUnwatch(c appengine.Context, m *xmpp.Message) {
    keyword := strings.ToLower(arg(m.Body, 1))
    room := username(m.To[0])
    
    if (!watchingKeyword(c, keyword, room)) {
        reply := &xmpp.Message{
            Sender: m.To[0],
            To: []string{m.Sender},
            Body: "Cannot unwatch unwatched keyword " + keyword,
        }
        reply.Send(c)
        return
    }
    
    q := datastore.NewQuery("Keyword").Filter("Value =", keyword).Filter("Room =", room).Limit(1)
	
    keywords := make([]Keyword, 0, 1)
    keys, _ := q.GetAll(c, &keywords)

    if len(keys) != 0 {
        datastore.Delete(c, keys[0])

        reply := &xmpp.Message{
            Sender: m.To[0],
            To: []string{m.Sender},
            Body: "Unwatching keyword " + keyword,
        }
        reply.Send(c)
    }
}

func watchedKeywords(c appengine.Context, m *xmpp.Message) []Keyword {
	room := username(m.To[0])
    q := datastore.NewQuery("Keyword").Filter("Room =", room).Limit(100)
    keywords := make([]Keyword, 0, 100)
    q.GetAll(c, &keywords)
    return keywords
}

func touchKeyword(c appengine.Context, m *xmpp.Message, keyword Keyword) {
	sender := bareJid(m.Sender)
	name := username(sender)

    q := datastore.NewQuery("Keyword").Filter("Room =", keyword.Room).Filter("Value=", keyword.Value).Limit(1)

    keywords := make([]Keyword, 0, 1)
    keys, _ := q.GetAll(c, &keywords)
    
    i := 0
    for _, dsKeyword := range keywords {
        dsKeyword.LastUsed = time.Now()
        dsKeyword.LastUser = name
        datastore.Put(c, keys[i], &dsKeyword)
        i++
    }
}

func handleWatchlist(c appengine.Context, m *xmpp.Message) {
    keywords := watchedKeywords(c, m)
    message := "Watching keywords: "
    
    for _, keyword := range keywords {
        message += fmt.Sprintf("%v, ", keyword.Value)
    }
    
    reply := &xmpp.Message{
        Sender: m.To[0],
        To: []string{m.Sender},
        Body: message,
    }
    reply.Send(c)
}

func handleKeyword(c appengine.Context, m *xmpp.Message) {
    keyword := arg(m.Body, 1)
	room := username(m.To[0])
    
    q := datastore.NewQuery("Keyword").Filter("Value =", keyword).Filter("Room =", room).Limit(1)
	
    keywords := make([]Keyword, 0, 1)
    keys, _ := q.GetAll(c, &keywords)

    if len(keys) != 0 {
        days := int(time.Duration.Hours(time.Now().Sub(keywords[0].LastUsed)) / 24)
        body := fmt.Sprintf("%d days since %v was last discussed. Keep up the good work. Only you can keep a safe, %s-free working environment!", days , strings.Title(keywords[0].Value), keywords[0].Value)
        announce(c, m, body, "announcement")
    } else {
        reply := &xmpp.Message{
            Sender: m.To[0],
            To: []string{m.Sender},
            Body: keyword + " not being watched.",
        }
        reply.Send(c)
    }
}

var commands = map[string]func(appengine.Context, *xmpp.Message)() {
    //TODO: Features - timeout : 3 blocks for a user results in a 5 minutes block.
    "help": handleHelp,
    "list": handleList,
    "meet": handleMeet,
    "leave": handleLeave,
    "pp": handlePingPong,
    "watch": handleWatch,
    "unwatch": handleUnwatch,
    "watchlist": handleWatchlist,
    "keyword": handleKeyword,
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
	        	To: []string{m.Sender},
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
	        To: []string{m.Sender},
	        Body: "Welcome to the room " + room + "!!!",
	    }
	    reply.Send(c)
	}
    
    if !command {
        keywords := watchedKeywords(c, m)
        bodyLower := strings.ToLower(m.Body)
        for _, keyword := range keywords {
            if (strings.Contains(bodyLower, keyword.Value)) {
                announce(c, m, fmt.Sprintf("It has been 0 days since %v was last discussed. The last person to bring it up was %v.", strings.Title(keyword.Value), keyword.LastUser), keyword.Value)
                touchKeyword(c, m, keyword)
            }
        }
    }
    
}
