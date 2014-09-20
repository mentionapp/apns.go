package apns

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"time"
)

type conn interface {
	Write(n *Notification) (connError bool, err error)
	Read() <-chan *ErrorResponse
	Done() <-chan struct{}
	Close()
	GetSentNotification(identifier NotificationIdentifier) *Notification
	GetSentNotificationsAfter(identifier NotificationIdentifier) []*Notification
	GetSentNotifications() []*Notification
	Expire()
}

type netConn struct {
	conn  net.Conn
	sent  *queue
	donec chan struct{}
	readc chan *ErrorResponse
}

// newConn creates a new conn instance
func newConn(addr string, cert *tls.Certificate) (conn, error) {

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

	conn := &netConn{
		conn:  tlsConn,
		sent:  q,
		donec: make(chan struct{}),
		readc: make(chan *ErrorResponse, 1),
	}

	go conn.read()

	return conn, nil
}

func (c *netConn) Write(n *Notification) (connError bool, err error) {

	payload, err := n.Encode()
	if err != nil {
		return false, fmt.Errorf("failed encoding notification %v: %v", n.Identifier(), err)
	}

	c.conn.SetWriteDeadline(time.Now().Add(time.Second * 60))
	if l, err := c.conn.Write(payload); err != nil {
		return true, fmt.Errorf("failed sending notification %v: %v", n.Identifier(), err)
	} else if l != len(payload) {
		return true, fmt.Errorf("failed sending notification %v: wrote %v bytes, expected %v", n.Identifier(), l, len(payload))
	}

	c.sent.Add(n)

	return false, nil
}

func (c *netConn) Read() <-chan *ErrorResponse {
	return c.readc
}

func (c *netConn) Done() <-chan struct{} {
	return c.donec
}

func (c *netConn) Close() {
	select {
	case <-c.donec:
	default:
		c.conn.Close()
		close(c.donec)
	}
}

func (c *netConn) GetSentNotification(identifier NotificationIdentifier) *Notification {
	return c.sent.Get(identifier)
}

func (c *netConn) GetSentNotificationsAfter(identifier NotificationIdentifier) []*Notification {
	return c.sent.GetAllAfter(identifier)
}

func (c *netConn) GetSentNotifications() []*Notification {
	return c.sent.GetAll()
}

func (c *netConn) Expire() {
	c.sent.Expire()
}

func (c *netConn) read() {

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
