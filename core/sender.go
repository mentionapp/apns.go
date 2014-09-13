package apns

import (
	"crypto/tls"
	"log"

	"code.google.com/p/go.net/context"
	"github.com/cenkalti/backoff"
)

// Sender sends notifications
type Sender struct {
	addr  string
	cert  *tls.Certificate
	conn  *conn
	pnc   chan *PushNotification
	pnrrc chan *PushNotificationRequestResponse
	err   chan *conn
}

// NewSender creates a new Sender
func NewSender(ctx context.Context, addr string, cert *tls.Certificate) *Sender {
	s := &Sender{
		addr:  addr,
		cert:  cert,
		pnc:   make(chan *PushNotification),
		pnrrc: make(chan *PushNotificationRequestResponse),
		err:   make(chan *conn),
	}
	go s.senderJob(ctx)
	return s
}

// Notifications returns the channel to which to send notifications
func (s *Sender) Notifications() chan *PushNotification {
	return s.pnc
}

// Responses returns the channel from which responses should be received
func (s *Sender) Responses() <-chan *PushNotificationRequestResponse {
	return s.pnrrc
}

func (s *Sender) senderJob(ctx context.Context) {

	handleErr := func(c *conn) {
		log.Printf("Error occured on connection, closing")
		s.conn.Close()
		if c == s.conn {
			s.conn = nil
		}
	}

	for {

		select {
		case c := <-s.err:
			handleErr(c)
		default:
		}

		select {
		case <-ctx.Done():
			if s.conn != nil {
				s.conn.Close()
			}
			return
		case c := <-s.err:
			handleErr(c)
		case pn := <-s.pnc:
			log.Printf("Sending notification %v", pn.Identifier)
			s.doSend(pn)
		}
	}
}

func (s *Sender) doSend(pn *PushNotification) {

	for {

		s.connect()

		payload, err := pn.ToBytes()
		if err != nil {
			// FIXME: user feedback
			log.Printf("Failed encoding notification %v: %v", pn.Identifier, err)
			return
		}

		if _, err = s.conn.Write(payload); err != nil {
			log.Printf("Failed sending notification %v: %v; will retry", pn.Identifier, err)
			s.conn.Close()
			s.conn = nil
			continue
		} else {
			s.conn.queue.Add(pn)
		}

		return
	}
}

func (s *Sender) connect() {

	for s.conn == nil {
		var conn *conn
		var err error

		connect := func() error {
			log.Printf("Connecting to %v", s.addr)
			conn, err = NewConn(s.addr, s.cert)
			if err != nil {
				log.Printf("Failed connecting to %v: %v; will retry", s.addr, err)
				return err
			}
			return nil
		}

		if backoff.Retry(connect, backoff.NewExponentialBackOff()) != nil {
			continue
		}

		go s.read(conn)

		s.conn = conn
	}
}

func (s *Sender) read(conn *conn) {
	buffer := make([]byte, 6)
	n, _ := conn.Read(buffer)

	s.err <- conn

	var pn *PushNotification
	var all []*PushNotification

	if n == len(buffer) {
		resp := &PushNotificationResponse{}
		resp.FromRawAppleResponse(buffer)

		pn = conn.queue.Get(resp.Identifier)

		if pn == nil {
			log.Printf("Got a response for unknown notification %v", resp.Identifier)
		} else {
			log.Printf("Got a response for notification %v", resp.Identifier)
			s.pnrrc <- &PushNotificationRequestResponse{
				Notification: pn,
				Response:     resp,
			}
		}
	}

	if pn != nil {
		all = conn.queue.GetAllAfter(pn.Identifier)
	} else {
		all = conn.queue.GetAll()
	}

	conn.queue.RemoveAll()

	for _, pn := range all {
		log.Printf("Requeuing notification %v", pn.Identifier)
		s.pnc <- pn
	}
}
