package apns

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"time"
)

type conn struct {
	conn  net.Conn
	sent  *queue
	donec chan struct{}
	readc chan *ErrorResponse
}

// newConn creates a new conn instance
func newConn(addr string, cert *tls.Certificate) (*conn, error) {

	c, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	name, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{*cert},
		ServerName:   name,
	}

	tlsConn := tls.Client(c, tlsConf)

	err = tlsConn.Handshake()
	if err != nil {
		c.Close()
		return nil, err
	}

	q := newQueue(time.Second * 60)

	conn := &conn{
		conn:  tlsConn,
		sent:  q,
		donec: make(chan struct{}),
		readc: make(chan *ErrorResponse, 1),
	}

	go conn.read()

	return conn, nil
}

func (c *conn) Write(pn *PushNotification) (connError bool, err error) {

	payload, err := pn.ToBytes()
	if err != nil {
		return false, fmt.Errorf("Failed encoding notification %v: %v", pn.Identifier, err)
	}

	c.conn.SetWriteDeadline(time.Now().Add(time.Second * 60))
	if n, err := c.conn.Write(payload); err != nil {
		return true, fmt.Errorf("Failed sending notification %v: %v", pn.Identifier, err)
	} else if n != len(payload) {
		return true, fmt.Errorf("Failed sending notification %v: wrote %v bytes, expected %v", pn.Identifier, n, len(payload))
	}

	c.sent.Add(pn)

	return false, nil
}

func (c *conn) Read() <-chan *ErrorResponse {
	return c.readc
}

func (c *conn) Done() <-chan struct{} {
	return c.donec
}

func (c *conn) Close() {
	select {
	case <-c.donec:
	default:
		c.conn.Close()
		close(c.donec)
	}
}

func (c *conn) GetSentNotification(identifier uint32) *PushNotification {
	return c.sent.Get(identifier)
}

func (c *conn) GetSentNotificationsAfter(identifier uint32) []*PushNotification {
	return c.sent.GetAllAfter(identifier)
}

func (c *conn) GetSentNotifications() []*PushNotification {
	return c.sent.GetAll()
}

func (c *conn) Expire() {
	c.sent.Expire()
}

func (c *conn) read() {

	var resp *ErrorResponse
	var err error

	buffer := make([]byte, ErrorResponseLength)
	n, _ := c.conn.Read(buffer)

	if n == len(buffer) {
		resp, err = DecodeErrorResponse(buffer)
		if err != nil {
			log.Printf("Failed decoding error-response: %v", err)
		}
	}

	c.readc <- resp
}
