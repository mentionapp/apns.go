package apns

import (
	"crypto/tls"
	"log"
	"time"

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
	readc chan *readEvent
}

type readEvent struct {
	buffer []byte
	n      int
	conn   *conn
}

// NewSender creates a new Sender
func NewSender(ctx context.Context, addr string, cert *tls.Certificate) *Sender {
	s := &Sender{
		addr:  addr,
		cert:  cert,
		pnc:   make(chan *PushNotification),
		pnrrc: make(chan *PushNotificationRequestResponse),
		readc: make(chan *readEvent),
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

	ticker := time.Tick(time.Second)

	for {

		select {
		case ev := <-s.readc:
			s.handleRead(ev)
		default:
		}

		select {
		case <-ctx.Done():
			if s.conn != nil {
				s.conn.Close()
			}
			return
		case ev := <-s.readc:
			s.handleRead(ev)
		case pn := <-s.pnc:
			log.Printf("Sending notification %v", pn.Identifier)
			s.doSend(pn)
		case <-ticker:
			if s.conn != nil {
				s.conn.queue.Expire()
			}
		}
	}
}

func (s *Sender) handleRead(ev *readEvent) {

	var pn *PushNotification
	var all []*PushNotification

	ev.conn.Close()
	if ev.conn == s.conn {
		s.conn = nil
	}

	if ev.n == len(ev.buffer) {
		resp := &PushNotificationResponse{}
		resp.FromRawAppleResponse(ev.buffer)

		pn = ev.conn.queue.Get(resp.Identifier)

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
		all = ev.conn.queue.GetAllAfter(pn.Identifier)
	} else {
		all = ev.conn.queue.GetAll()
	}

	ev.conn.queue.RemoveAll()

	go func() {
		for _, pn := range all {
			log.Printf("Requeuing notification %v", pn.Identifier)
			s.pnc <- pn
		}
	}()
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

	s.readc <- &readEvent{
		buffer: buffer,
		n:      n,
		conn:   conn,
	}
}
