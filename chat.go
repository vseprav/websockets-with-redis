package main

import (
	"net/http"
	"html/template"
	"golang.org/x/net/websocket"
	redis "gopkg.in/redis.v4"
	"time"
	"os"
)

const templatesPath string = "templates/"

var (
	client *redis.Client
	pubsub *redis.PubSub
	redis_err error
	msgChan = make(chan string, 100)
	clientRequests = make(chan *ClientRequest, 100)
	clientDisconnects   = make(chan time.Time, 100)
)

type (

	ClientRequest struct {
		clientKey time.Time
		conn   *websocket.Conn
	}
)

func main() {

	client = redis.NewClient(&redis.Options{
		Addr:     "192.168.1.236:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	pubsub, redis_err = client.Subscribe("chatChanel"); if redis_err != nil {
		panic(redis_err)
	}

	defer func() {
		pubsub.Close()
	}()

	go receiveMessage()
	go sendMessages()

	http.HandleFunc("/", handleIndexPage)
	http.Handle("/chat", websocket.Handler(chatServer))

	serverPort := os.Args[1]

	err := http.ListenAndServe(serverPort, nil)
	if err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}

func receiveMessage() {
	for {
		msg, err := pubsub.ReceiveMessage(); if err != nil {
			panic(err)
		}
		msgChan <- msg.Payload
	}
}

func sendMessages() {
	var connections = make(map[time.Time]*websocket.Conn)
	for {
		select {
		case req := <-clientRequests:
			connections[req.clientKey] = req.conn
		case msg := <-msgChan:
			for _, con := range connections {
				websocket.Message.Send(con, msg)
			}
		case clientKey := <-clientDisconnects:
			delete(connections, clientKey)
		}
	}
}


func chatServer(ws *websocket.Conn) {
	var clientKey = time.Now()
	clientRequests <- &ClientRequest{clientKey, ws}

	var message string
	for {
		err := websocket.Message.Receive(ws, &message)
		if err == nil {
			client.Publish("chatChanel", message).Err()
		} else {
			ws.Close()
			clientDisconnects <- clientKey
		}
	}
}

func handleIndexPage(w http.ResponseWriter, r *http.Request)  {
	indexPage := template.Must(template.ParseFiles(templatesPath + "index.html"))

	if err := indexPage.ExecuteTemplate(w, "index.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
