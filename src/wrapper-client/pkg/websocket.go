package wrapper

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/layer-devops/wrap.sh/src/protocol"
	"github.com/pkg/errors"
	"io"
	"log"
	"net/http"
	"time"
)

func (client *Client) read() ([]byte, error) {
	client.recvBuf.Reset()
	messageType, reader, err := client.ws.NextReader()
	if err != nil {
		return nil, err
	}
	if messageType != websocket.BinaryMessage {
		return nil, fmt.Errorf("unexpected message type: %v", messageType)
	}
	_, err = io.Copy(&client.recvBuf, reader)
	return client.recvBuf.Bytes(), err
}

func (client *Client) send(msg *protocol.MessageFromWrapperClient) error {
	client.wsWriteMutex.Lock()
	defer client.wsWriteMutex.Unlock()
	b, err := proto.Marshal(msg)
	if err != nil {
		return errors.Wrap(err, "could not encode message to server")
	}
	client.ws.SetWriteDeadline(time.Now().Add(time.Second * 10))
	err = client.ws.WriteMessage(websocket.BinaryMessage, b)
	return errors.Wrap(err, "could not write message to server")
}

func (client *Client) connectToServer() error {
	client.debugLog("connecting to %s", client.WebsocketLocation)
	if client.LogDebug {
		if client.Token == "" {
			client.debugLog("no auth token")
		} else {
			client.debugLog("auth token: %v", client.Token)
		}
	}
	header := http.Header{}
	header.Set(protocol.WrapperAuthHeaderName, client.Token)
	ws, response, err := websocket.DefaultDialer.Dial(client.WebsocketLocation, header)
	if err != nil {
		if response != nil {
			client.debugLog("got %v for status code while dialing", response.StatusCode)
		}
		if response != nil && response.StatusCode == http.StatusNotFound {
			return errors.New("could not authenticate with wrap.sh server")
		}
		return errors.Wrap(err, "dial wrap.sh server")
	}
	client.ws = ws
	return client.sendHello()
}

func (client *Client) listenServer() {
	for {
		if client.closed {
			return
		}
		b, err := client.read()
		if err != nil {
			if errors.Cause(err) == io.EOF {
				client.close()
			} else if wsErr, ok := errors.Cause(err).(*websocket.CloseError); ok {
				if wsErr.Code == websocket.CloseGoingAway || wsErr.Code == websocket.CloseAbnormalClosure {
					client.close()
				} else {
					client.Log(errors.Wrap(err, "connection error").Error())
					client.close()
				}
			} else {
				client.Log(errors.Wrap(err, "invalid message data received").Error())
				client.close()
			}
			return
		}
		msg := &protocol.MessageToWrapperClient{}
		err = proto.Unmarshal(b, msg)
		err = errors.Wrap(err, "could not parse message from wrapper server")
		if err != nil {
			log.Fatal(err)
		}
		err = client.HandleMessage(msg)
	}
}
