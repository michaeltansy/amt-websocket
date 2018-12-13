package amtsocket

import (
	"crypto/tls"
	"errors"
	"log"
	"net/http"
	"net/url"
	"sync"

	uuid "github.com/satori/go.uuid"

	"github.com/gorilla/websocket"
)

type Socket struct {
	ID                string
	Conn              *websocket.Conn
	WebsocketDialer   *websocket.Dialer
	Url               string
	ConnectionOptions ConnectionOptions
	RequestHeader     http.Header
	OnConnected       func(socket Socket)
	OnTextMessage     func(message string, socket Socket)
	OnBinaryMessage   func(data []byte, socket Socket)
	OnConnectError    func(err error, socket Socket)
	OnDisconnected    func(err error, socket Socket)
	OnPingReceived    func(data string, socket Socket)
	OnPongReceived    func(data string, socket Socket)
	IsConnected       bool
	sendMutex         *sync.Mutex // Prevent "concurrent write to websocket connection"
	receiveMutex      *sync.Mutex
}

type ConnectionOptions struct {
	UseCompression bool
	UseSSL         bool
	Proxy          func(*http.Request) (*url.URL, error)
	Subprotocols   []string
}

type SocketEvent interface {
	OnPingReceived(data string, socket Socket)
	OnPongReceived(data string, socket Socket)
	OnConnected(socket Socket)
	OnDisconnected(err error, socket Socket)
	OnConnectError(err error, socket Socket)
	OnTextMessage(message string, socket Socket)
	OnBinaryMessage(data []byte, socket Socket)
}

// todo Yet to be done
type ReconnectionOptions struct {
}

// New create new socket connection
func New(url string) Socket {
	return Socket{
		Url:           url,
		RequestHeader: http.Header{},
		ConnectionOptions: ConnectionOptions{
			UseCompression: false,
			UseSSL:         true,
		},
		WebsocketDialer: &websocket.Dialer{},
		sendMutex:       &sync.Mutex{},
		receiveMutex:    &sync.Mutex{},
	}
}

// setConnectionOptions set
func (socket *Socket) setConnectionOptions() {
	socket.WebsocketDialer.EnableCompression = socket.ConnectionOptions.UseCompression
	socket.WebsocketDialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: socket.ConnectionOptions.UseSSL}
	socket.WebsocketDialer.Proxy = socket.ConnectionOptions.Proxy
	socket.WebsocketDialer.Subprotocols = socket.ConnectionOptions.Subprotocols
}

// SetID set socketID of a connected user. It generates uuid as socketID by default, however, it also accepts customID
func (socket *Socket) SetID(customID ...string) {
	var ID string
	if len(customID) > 0 {
		ID = customID[0]
	} else {
		uuid, _ := uuid.NewV4()
		ID = uuid.String()
	}

	socket.ID = ID
}

// GetID returns socketID of a connected user.
func (socket *Socket) GetID() string {
	return socket.ID
}

// SetConnectionStatus set connection status (true:connected, false:disconnected)
func (socket *Socket) SetConnectionStatus(status bool) {
	socket.IsConnected = status
}

// GetConnectionStatus get connection status (true:connected, false:disconnected)
func (Socket *Socket) GetConnectionStatus() bool {
	return Socket.IsConnected
}

// Connect connect socket
func (socket *Socket) Connect() {
	var err error
	socket.setConnectionOptions()

	socket.Conn, _, err = socket.WebsocketDialer.Dial(socket.Url, socket.RequestHeader)
	if err != nil {
		log.Println(err)
		socket.SetConnectionStatus(false)
		if socket.OnConnectError != nil {
			socket.OnConnectError(err, *socket)
		}
		return
	}

	log.Println("Socket connected in server:", socket.Url)

	if socket.OnConnected != nil {
		socket.SetConnectionStatus(true)
		socket.OnConnected(*socket)
	}

	defaultPingHandler := socket.Conn.PingHandler()
	socket.Conn.SetPingHandler(func(appData string) error {
		log.Println("Received PING from server")
		if socket.OnPingReceived != nil {
			socket.OnPingReceived(appData, *socket)
		}
		return defaultPingHandler(appData)
	})

	defaultPongHandler := socket.Conn.PongHandler()
	socket.Conn.SetPongHandler(func(appData string) error {
		log.Println("Received PONG from server")
		if socket.OnPongReceived != nil {
			socket.OnPongReceived(appData, *socket)
		}
		return defaultPongHandler(appData)
	})

	defaultCloseHandler := socket.Conn.CloseHandler()
	socket.Conn.SetCloseHandler(func(code int, text string) error {
		result := defaultCloseHandler(code, text)
		log.Println("Disconnected from server ", result)
		if socket.OnDisconnected != nil {
			socket.SetConnectionStatus(false)
			socket.OnDisconnected(errors.New(text), *socket)
		}
		return result
	})

	go func() {
		for {
			socket.receiveMutex.Lock()
			messageType, message, err := socket.Conn.ReadMessage()
			socket.receiveMutex.Unlock()
			if err != nil {
				log.Println("read:", err)
				return
			}
			log.Println("recv: ", string(message))

			switch messageType {
			case websocket.TextMessage:
				if socket.OnTextMessage != nil {
					socket.OnTextMessage(string(message), *socket)
				}
			case websocket.BinaryMessage:
				if socket.OnBinaryMessage != nil {
					socket.OnBinaryMessage(message, *socket)
				}
			}
		}
	}()
}

// SendText send message as string
func (socket *Socket) SendText(message string) {
	err := socket.send(websocket.TextMessage, []byte(message))
	if err != nil {
		log.Println("write:", err)
		return
	}
}

// SendBinary send message as []byte
func (socket *Socket) SendBinary(data []byte) {
	err := socket.send(websocket.BinaryMessage, data)
	if err != nil {
		log.Println("write:", err)
		return
	}
}

// send send message
func (socket *Socket) send(messageType int, data []byte) error {
	socket.sendMutex.Lock()
	err := socket.Conn.WriteMessage(messageType, data)
	socket.sendMutex.Unlock()
	return err
}

// Close close socket connection
func (socket *Socket) Close() {
	err := socket.send(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		log.Println("write close:", err)
	}
	socket.Conn.Close()
	if socket.OnDisconnected != nil {
		socket.SetConnectionStatus(false)
		socket.OnDisconnected(err, *socket)
	}
}
