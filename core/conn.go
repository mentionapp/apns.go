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

	return conn, nil
}

func (c *conn) Close() {
	select {
	case <-c.done:
	default:
		c.Conn.Close()
		close(c.done)
	}
}
