package apns

import (
	"crypto/tls"
	"net"
	"time"
)

type conn struct {
	net.Conn
	queue *queue
	done  chan struct{}
}

// NewConn creates a new conn instance
func NewConn(addr string, cert *tls.Certificate) (*conn, error) {

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

	q := NewQueue(time.Second * 60)

	conn := &conn{
		Conn:  tlsConn,
		queue: q,
		done:  make(chan struct{}),
	}

	go conn.expire()

	return conn, nil
}

func (c *conn) Close() {
	c.Conn.Close()
	close(c.done)
}

func (c *conn) expire() {
	ticker := time.Tick(time.Second)
	for {
		select {
		case <-c.done:
			break
		case <-ticker:
			c.queue.Expire()
		}
	}
}
