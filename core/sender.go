package apns

import (
	"crypto/tls"
	"fmt"
	"log"
	"time"

	"code.google.com/p/go.net/context"
	"github.com/cenkalti/backoff"
)

// Sender sends notifications
type Sender struct {
	addr       string
	cert       *tls.Certificate
	conn       *conn
	notifc     chan *PushNotification
	prioNotifc *priochan
	respc      chan *PushNotificationRequestResponse
	readc      chan *readEvent
}

type readEvent struct {
	resp *PushNotificationResponse
	conn *conn
}

// NewSender creates a new Sender
func NewSender(ctx context.Context, addr string, cert *tls.Certificate) *Sender {
	s := &Sender{
		addr:       addr,
		cert:       cert,
		notifc:     make(chan *PushNotification),
		prioNotifc: newPriochan(),
		respc:      make(chan *PushNotificationRequestResponse),
		readc:      make(chan *readEvent),
	}

	s.prioNotifc.Add(s.notifc)

	go s.senderJob(ctx)

	return s
}

// Notifications returns the channel to which to send notifications
func (s *Sender) Notifications() chan *PushNotification {
	return s.notifc
}

// Responses returns the channel from which responses should be received
func (s *Sender) Responses() <-chan *PushNotificationRequestResponse {
	return s.respc
}

func (s *Sender) senderJob(ctx context.Context) {

	ticker := time.Tick(time.Second)

	for {
		select {
		case <-ctx.Done():
			if s.conn != nil {
				s.conn.Close()
			}
			s.prioNotifc.Close()
			return
		case ev := <-s.readc:
			s.handleRead(ev)
		case pn := <-s.prioNotifc.Receive():
			log.Printf("Sending notification %v", pn.Identifier)
			s.doSend(pn)
		case <-ticker:
			if s.conn != nil {
				s.conn.Expire()
			}
		}
	}
}

func (s *Sender) handleRead(ev *readEvent) {

	var pn *PushNotification
	var sent []*PushNotification
	conn := ev.conn

	conn.Close()
	if conn == s.conn {
		s.conn = nil
	}

	if resp := ev.resp; resp != nil {
		pn = conn.GetSentNotification(resp.Identifier)

		if pn == nil {
			log.Printf("Got a response for unknown notification %v", resp.Identifier)
		} else {
			log.Printf("Got a response for notification %v", resp.Identifier)
			s.respc <- &PushNotificationRequestResponse{
				Notification: pn,
				Response:     resp,
			}
		}
	}

	if pn != nil {
		sent = conn.GetSentNotificationsAfter(pn.Identifier)
	} else {
		sent = conn.GetSentNotifications()
	}

	// requeue notifications before anything sent to s.notifc
	c := make(chan *PushNotification)
	s.prioNotifc.Add(c)

	go func() {
		for _, pn := range sent {
			log.Printf("Requeuing notification %v", pn.Identifier)
			c <- pn
		}
	}()
}

func (s *Sender) doSend(pn *PushNotification) {

	for {
		s.connect()

		if connError, err := s.conn.Write(pn); err != nil {
			if connError {
				s.conn.Close()
				s.conn = nil
				fmt.Printf("%v; will retry", err)
			} else {
				fmt.Println(err)
			}
		} else {
			break
		}
	}
}

func (s *Sender) connect() {

	for s.conn == nil {
		var conn *conn
		var err error

		connect := func() error {
			log.Printf("Connecting to %v", s.addr)
			conn, err = newConn(s.addr, s.cert)
			if err != nil {
				log.Printf("Failed connecting to %v: %v; will retry", s.addr, err)
				return err
			}
			return nil
		}

		if backoff.Retry(connect, backoff.NewExponentialBackOff()) != nil {
			continue
		}

		log.Printf("Connected to %v", s.addr)

		go s.read(conn)

		s.conn = conn
	}
}

func (s *Sender) read(c *conn) {
	for {
		select {
		case <-c.Done():
			return
		case pnr := <-c.Read():
			s.readc <- &readEvent{pnr, c}
		}
	}
}
