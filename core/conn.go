package apns

import (
	"crypto/tls"
	"fmt"
	"net"
	"time"
)

type conn struct {
	conn  net.Conn
	sent  *queue
	donec chan struct{}
	readc chan *PushNotificationResponse
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
		readc: make(chan *PushNotificationResponse, 1),
	}

	go conn.read()

	return conn, nil
}

func (c *conn) Write(pn *PushNotification) (connError bool, err error) {

	payload, err := pn.ToBytes()
	if err != nil {
		return false, fmt.Errorf("Failed encoding notification %v: %v", pn.Identifier, err)
	}

	if _, err = c.conn.Write(payload); err != nil {
		return true, fmt.Errorf("Failed sending notification %v: %v", pn.Identifier, err)
	}

	c.sent.Add(pn)

	return false, nil
}

func (c *conn) Read() <-chan *PushNotificationResponse {
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

	buffer := make([]byte, 6)
	n, _ := c.conn.Read(buffer)

	var resp *PushNotificationResponse

	if n == len(buffer) {
		resp = &PushNotificationResponse{}
		resp.FromRawAppleResponse(buffer)
	}

	c.readc <- resp
}
