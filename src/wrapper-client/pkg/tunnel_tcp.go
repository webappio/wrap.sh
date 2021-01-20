package wrapper

import (
	"github.com/layer-devops/wrap.sh/src/protocol"
	"github.com/pkg/errors"
	"net"
	"sync"
	"time"
)

type tunnelTcpConn struct {
	Id           uint32
	Conn         *net.TCPConn
	closed       bool
	closingMutex sync.Mutex
}

func (tc *tunnelTcpConn) Close() {
	tc.closingMutex.Lock()
	defer tc.closingMutex.Unlock()
	if tc.closed {
		return
	}
	tc.closed = true
	_ = tc.Conn.Close()
}

func (client *Client) handleTcpReadCall(msg *protocol.TcpReadMessage) error {
	connId := msg.GetConnectionId()
	readerId := msg.GetReaderId()
	if tc, ok := client.connections[connId]; ok && !tc.closed {
		go func() {
			b := make([]byte, msg.GetBufferSize())
			n, err := tc.Conn.Read(b)
			errMsg := ""
			if err != nil {
				errMsg = err.Error()
			}
			err = client.send(&protocol.MessageFromWrapperClient{
				Spec: &protocol.MessageFromWrapperClient_TcpReadResult{
					TcpReadResult: &protocol.TcpReadResultMessage{
						ConnectionId: connId,
						ReaderId:     readerId,
						BytesRead:    uint32(n),
						Data:         b,
						Error:        errMsg,
					},
				},
			})
			if err != nil {
				panic(errors.Wrap(err, "could not send write response"))
			}
		}()
	} else {
		return errors.New("read call for unknown or closed connection")
	}
	return nil
}

func (client *Client) handleTcpWriteCall(msg *protocol.TcpWriteMessage) error {
	connId := msg.GetConnectionId()
	writerId := msg.GetWriterId()
	if tc, ok := client.connections[connId]; ok && !tc.closed {
		b := msg.GetData()
		go func() {
			n, err := tc.Conn.Write(b)
			errMsg := ""
			if err != nil {
				errMsg = err.Error()
			}
			err = client.send(&protocol.MessageFromWrapperClient{
				Spec: &protocol.MessageFromWrapperClient_TcpWriteResult{
					TcpWriteResult: &protocol.TcpWriteResultMessage{
						ConnectionId: connId,
						WriterId:     writerId,
						BytesWritten: uint32(n),
						Error:        errMsg,
					},
				},
			})
			if err != nil {
				panic(errors.Wrap(err, "could not send write response"))
			}
		}()
	} else {
		return errors.New("write call for unknown or closed connection")
	}
	return nil
}

func (client *Client) handleTcpDialCall(msg *protocol.TcpDialMessage) error {
	connectionId := msg.GetConnectionId()
	if client.connections == nil {
		client.connections = map[uint32]*tunnelTcpConn{}
	}
	if _, exists := client.connections[connectionId]; exists {
		return errors.New("dial for existing connection")
	}
	// establish a new connection, notify wrapper server of the result,
	// then listen on the connection
	address := msg.GetAddress()
	go func() {
		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}

		conn, err := dialer.Dial("tcp", address)
		if err != nil {
			// e.g. could not resolve the address; report the issue to the wrapper server
			client.debugLog(errors.Wrap(err, "dial tcp").Error())
			err = client.send(&protocol.MessageFromWrapperClient{
				Spec: &protocol.MessageFromWrapperClient_TcpDialResult{
					TcpDialResult: &protocol.TcpDialResultMessage{
						ConnectionId: connectionId,
						Error:        err.Error(),
					},
				},
			})
			if err != nil {
				panic(errors.Wrap(err, "could not send dial result"))
			}
			return
		}
		tc := &tunnelTcpConn{
			Id:   connectionId,
			Conn: conn.(*net.TCPConn),
		}
		client.connMapWriteMutex.Lock()
		client.connections[connectionId] = tc
		client.connMapWriteMutex.Unlock()
		err = client.send(&protocol.MessageFromWrapperClient{
			Spec: &protocol.MessageFromWrapperClient_TcpDialResult{
				TcpDialResult: &protocol.TcpDialResultMessage{
					ConnectionId: connectionId,
					Address:      conn.RemoteAddr().String(),
				},
			},
		})
		if err != nil {
			panic(errors.Wrap(err, "could not send dial result"))
		}
	}()
	return nil
}
