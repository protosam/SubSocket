package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func main() {

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static", fs))

	http.HandleFunc("/socket", openSocket)

	http.HandleFunc("/api.v1/channels/list", listChannels)
	http.HandleFunc("/api.v1/channels/put", putChannel)
	http.HandleFunc("/api.v1/channels/del", delChannel)
	http.HandleFunc("/api.v1/clients/subscribe", subscribeClient)
	http.HandleFunc("/api.v1/clients/unsubscribe", unsubscribeClient)

	http.ListenAndServe(":8080", nil)
}

func subscribeClient(w http.ResponseWriter, r *http.Request) {
	channel, ok := r.URL.Query()["channel"]
	if !ok || len(channel) < 1 {
		fmt.Fprintf(w, "channel are required input...\n")
		return
	}
	client_id, ok := r.URL.Query()["client-id"]
	if !ok || len(client_id) < 1 {
		fmt.Fprintf(w, "client-id are required input...\n")
		return
	}

	data := strings.SplitN("sub "+channel[0], " ", 2)
	if len(data) != 2 {
		fmt.Fprintf(w, "No channel specified\n")
		return
	}

	socket_subscribe(client_id[0], data, false)
}

func unsubscribeClient(w http.ResponseWriter, r *http.Request) {
	channel, ok := r.URL.Query()["channel"]
	if !ok || len(channel) < 1 {
		fmt.Fprintf(w, "channel are required input...\n")
		return
	}
	client_id, ok := r.URL.Query()["client-id"]
	if !ok || len(client_id) < 1 {
		fmt.Fprintf(w, "client-id are required input...\n")
		return
	}

	data := strings.SplitN("sub "+channel[0], " ", 2)
	if len(data) != 2 {
		fmt.Fprintf(w, "No channel specified\n")
		return
	}

	socket_unsubscribe(client_id[0], data, false)
}
func listChannels(w http.ResponseWriter, r *http.Request) {
	// Admin required.
	if r.Header.Get("Admin-Token") != AdminToken {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	channels_mtx.Lock()
	for name := range channels {
		fmt.Fprintf(w, name+"\n")
	}
	channels_mtx.Unlock()
}

func putChannel(w http.ResponseWriter, r *http.Request) {
	// Admin required.
	if r.Header.Get("Admin-Token") != AdminToken {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	ch_names, ok := r.URL.Query()["name"]
	if !ok || len(ch_names) < 1 {
		fmt.Fprintf(w, "names are required input...\n")
		return
	}
	ch_is_public, ok := r.URL.Query()["is-public"]
	if !ok || len(ch_is_public) != len(ch_names) {
		fmt.Fprintf(w, "is-public fields are required for each name input...\n")
		return
	}
	ch_allow_broadcast, ok := r.URL.Query()["allow-broadcast"]
	if !ok || len(ch_allow_broadcast) != len(ch_names) {
		fmt.Fprintf(w, "allow-broadcast fields are required for each name input...\n")
		return
	}

	// just always create the channel if it doesn't exist. new keys will be generated
	for i, ch_name := range ch_names {
		ch := Channel{}

		if strings.ToLower(ch_is_public[i]) == "yes" {
			ch.IsPublic = true
		}
		if strings.ToLower(ch_allow_broadcast[i]) == "yes" {
			ch.AllowBroadcast = true
		}

		ch.Subscribers = make(map[string]*SocketClient)

		channels_mtx.Lock()
		channels[ch_name] = ch
		channels_mtx.Unlock()

		fmt.Fprintf(w, ch_name+" has been put\n")
	}
}

func delChannel(w http.ResponseWriter, r *http.Request) {
	// Admin required.
	if r.Header.Get("Admin-Token") != AdminToken {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	ch_names, ok := r.URL.Query()["name"]
	if !ok || len(ch_names) < 1 {
		fmt.Fprintf(w, "names are required input...\n")
		return
	}

	// create it if it doesn't exist
	for _, ch_name := range ch_names {
		channels_mtx.Lock()
		delete(channels, ch_name)
		channels_mtx.Unlock()
		fmt.Fprintf(w, ch_name+" has been deleted\n")
	}
}

/* PubSub management code
 */

type Channel struct {
	IsPublic       bool // Can people subscribe to read from channel without IsAdmin?
	AllowBroadcast bool // Can people broadcast to this channel without IsAdmin?
	Subscribers    map[string]*SocketClient
	Mutex          sync.Mutex
}

// This structure is data about the connected user
type SocketClient struct {
	Id            string   // Unique ID used by server to identify client in code
	Subscriptions []string // channels subscribed to
	Connection    *websocket.Conn
	IsAdmin       bool
	Mutex         sync.Mutex
}

// This is used to create/delete channels
const AdminToken = "123456"

// Channel DB
var channels = make(map[string]Channel)
var channels_mtx = sync.Mutex{}

// Connected user DB
var connected = make(map[string]*SocketClient)
var connected_mtx = sync.Mutex{}

/* WebSocket code...
 */
var upgrader = websocket.Upgrader{} // use default options

func closeSocket(c *SocketClient) {
	// Remove the user from their subscriptions
	for _, subscription := range c.Subscriptions {
		if _, ok := channels[subscription]; ok {
			channels_mtx.Lock()
			delete(channels[subscription].Subscribers, c.Id)
			channels_mtx.Unlock()
		}
	}

	connected_mtx.Lock()
	// delete the user from connections
	delete(connected, c.Id)

	// Clear the user from the connected list
	delete(connected, c.Id)
	connected_mtx.Unlock()

}

func openSocket(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()

	sclient := &SocketClient{}
	id := uuid.New().String()
	sclient.Id = id
	sclient.Mutex = sync.Mutex{}
	sclient.Connection = c

	connected_mtx.Lock()
	connected[id] = sclient
	connected_mtx.Unlock()
	socket_notify(id, "CONNECTED")

	defer socket_notify(id, "DISCONNECTED")
	defer closeSocket(sclient)

	// add user to connection db
	//conns = append(conns, c)
	// User starts off with no subscriptions.
	// User has 30 seconds to subscribe or receive the boot

	for {
		fmt.Println("waiting for msg")
		mt, raw_data, err := c.ReadMessage()
		fmt.Println("msg received", mt)
		if err != nil {
			log.Println("read:", err)
			break
		}
		// Parse the message received
		// All messages are in format of "command [parameters...]"
		data := strings.SplitN(string(raw_data), " ", 2)
		switch data[0] {
		case "unsub":
			socket_unsubscribe(sclient.Id, data, true)
		case "sub":
			socket_subscribe(sclient.Id, data, true)
		case "admin-token":
			socket_admin_token(sclient, data)
		case "pub":
			socket_publish(sclient, data)
		case "broadcast":
			socket_broadcast(sclient, data)
		case "message":
			socket_message(sclient, data)
		}
	}
}

// message UUID MESSAGE
func socket_message(sclient *SocketClient, data []string) {
	if len(data) != 2 || !sclient.IsAdmin {
		return
	}

	data = strings.SplitN(data[1], " ", 2)
	if len(data) != 2 {
		return
	}
	connected_mtx.Lock()
	if _, ok := connected[data[0]]; ok {
		connected[data[0]].Mutex.Lock()
		connected[data[0]].Connection.WriteMessage(1, []byte("__system "+sclient.Id+" "+data[1]))
		connected[data[0]].Mutex.Unlock()
	}
	connected_mtx.Unlock()
}

// broadcast MESSAGE
func socket_broadcast(sclient *SocketClient, data []string) {
	if len(data) != 2 || !sclient.IsAdmin {
		return
	}

	connected_mtx.Lock()
	for i := range connected {
		connected[i].Mutex.Lock()
		connected[i].Connection.WriteMessage(1, []byte("__broadcast "+sclient.Id+" "+data[1]))
		connected[i].Mutex.Unlock()
	}
	connected_mtx.Unlock()
}

// admin client notification (internal only)
func socket_notify(id string, message string) {
	connected_mtx.Lock()
	for i := range connected {
		if connected[i].IsAdmin {
			connected[i].Mutex.Lock()
			connected[i].Connection.WriteMessage(1, []byte("__notify "+id+" "+message))
			connected[i].Mutex.Unlock()
		}
	}
	connected_mtx.Unlock()
}

// pub CHANNEL MESSAGE
func socket_publish(sclient *SocketClient, data []string) {
	if len(data) != 2 {
		return
	}

	data = strings.SplitN(data[1], " ", 2)
	if len(data) != 2 {
		return
	}

	channels_mtx.Lock()
	if _, ok := channels[data[0]]; ok && (channels[data[0]].AllowBroadcast || sclient.IsAdmin) {
		for i := range channels[data[0]].Subscribers {
			channels[data[0]].Subscribers[i].Mutex.Lock()
			fmt.Println(i)
			channels[data[0]].Subscribers[i].Connection.WriteMessage(1, []byte(data[0]+" "+sclient.Id+" "+data[1]))
			channels[data[0]].Subscribers[i].Mutex.Unlock()
		}
	}
	channels_mtx.Unlock()
}

// admin-token TOKEN
func socket_admin_token(sclient *SocketClient, data []string) {
	if len(data) != 2 {
		return
	}

	if data[1] == AdminToken {
		sclient.IsAdmin = true
	}
}

// sub CHANNEL
func socket_subscribe(client_id string, data []string, check_private bool) {
	if len(data) != 2 {
		return
	}

	// Lookup by client_id
	connected_mtx.Lock()
	sclient, ok := connected[client_id]
	connected_mtx.Unlock()
	if !ok {
		return
	}
	fmt.Println("FOUND 1")
	if _, ok := channels[data[1]]; ok {
		fmt.Println("FOUND 2")
		if !sclient.IsAdmin {
			if check_private && !channels[data[1]].IsPublic {
				return
			}
		}
		fmt.Println("FOUND 3 " + data[1])
		sclient.Mutex.Lock()
		channels_mtx.Lock()
		channels[data[1]].Subscribers[sclient.Id] = sclient
		channels_mtx.Unlock()
		sclient.Mutex.Unlock()
	}
}

// unsub CHANNEL
func socket_unsubscribe(client_id string, data []string, check_private bool) {
	if len(data) != 2 {
		return
	}

	// Lookup by client_id
	connected_mtx.Lock()
	sclient, ok := connected[client_id]
	connected_mtx.Unlock()
	if !ok {
		return
	}

	if _, ok := channels[data[1]]; ok {
		if check_private && !channels[data[1]].IsPublic {
			return
		}
		sclient.Mutex.Lock()
		channels_mtx.Lock()
		delete(channels[data[1]].Subscribers, sclient.Id)
		channels_mtx.Unlock()
		sclient.Mutex.Unlock()
	}
}
