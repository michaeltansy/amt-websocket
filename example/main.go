package main

import (
	"log"
	"os"
	"os/signal"

	amtsocket "github.com/michaeltansy/amt-websocket"
)

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	socket := amtsocket.New("ws://echo.websocket.org/")
	socket.ConnectionOptions = amtsocket.ConnectionOptions{
		//Proxy: amtsocket.BuildProxy("http://example.com"),
		UseSSL:         false,
		UseCompression: false,
		Subprotocols:   []string{"chat", "superchat"},
	}

	socket.RequestHeader.Set("Accept-Encoding", "gzip, deflate, sdch")
	socket.RequestHeader.Set("Accept-Language", "en-US,en;q=0.8")
	socket.RequestHeader.Set("Pragma", "no-cache")
	socket.RequestHeader.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/49.0.2623.87 Safari/537.36")

	socket.OnConnectError = func(err error, socket amtsocket.Socket) {
		log.Fatal("Recieved connect error ", err)
	}
	socket.OnConnected = func(socket amtsocket.Socket) {
		log.Println("Connected to server")
	}
	socket.OnTextMessage = func(message string, socket amtsocket.Socket) {
		log.Println("Recieved message  " + message)
	}
	socket.OnPingReceived = func(data string, socket amtsocket.Socket) {
		log.Println("Recieved ping " + data)
	}
	socket.OnDisconnected = func(err error, socket amtsocket.Socket) {
		log.Println("Disconnected from server ")
		return
	}
	socket.Connect()

	i := 0
	for i < 10 {
		socket.SendText("This is my sample test message")
		i++
	}

	for {
		select {
		case <-interrupt:
			log.Println("interrupt")
			socket.Close()
			return
		}
	}
}
