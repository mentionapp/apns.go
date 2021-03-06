package apns

import (
	"crypto/tls"
	"log"
	"time"

	"github.com/cenkalti/backoff"
	"golang.org/x/net/context"
)

const (
	SenderGateway        string = "gateway.push.apple.com:2195"
	SenderSandboxGateway string = "gateway.sandbox.push.apple.com:2195"
)

var Verbose bool

// Sender sends notifications
type Sender struct {
	addr       string
	cert       *tls.Certificate
	conn       conn
	notifc     chan *Notification
	prioNotifc *priochan
	errorc     chan *SenderError
	readc      chan *readEvent
	newConn    func(addr string, cert *tls.Certificate) (conn, error)
	donec      chan struct{}
	nextId     NotificationIdentifier
}

// SenderError represents a sender error
type SenderError struct {
	Notification  *Notification
	ErrorResponse *ErrorResponse
}

type readEvent struct {
	resp *ErrorResponse
	conn conn
}

func info(format string, v ...interface{}) {
	if Verbose {
		log.Printf(format, v...)
	}
}

// NewSender creates a new Sender
func NewSender(ctx context.Context, addr string, cert *tls.Certificate) *Sender {
	s := &Sender{
		addr:       addr,
		cert:       cert,
		notifc:     make(chan *Notification),
		prioNotifc: newPriochan(),
		errorc:     make(chan *SenderError),
		readc:      make(chan *readEvent),
		newConn:    newConn,
		donec:      make(chan struct{}),
	}

	s.prioNotifc.Add(s.notifc)

	go s.senderJob(ctx)

	return s
}

// Notifications returns the channel to which to send notifications
func (s *Sender) Notifications() chan *Notification {
	return s.notifc
}

// Errors returns the channel from which to receive SenderErrors
func (s *Sender) Errors() <-chan *SenderError {
	return s.errorc
}

// Done returns a channel that's closed when this Sender has terminated (usually
// after ctx.Done() has been closed).
func (s *Sender) Done() <-chan struct{} {
	return s.donec
}

func (s *Sender) senderJob(ctx context.Context) {
	ticker := time.Tick(time.Second)

start:
	for {
		select {
		case <-ctx.Done():
			if s.conn != nil {
				s.conn.Close()
			}
			s.prioNotifc.Close()
			break start
		case ev := <-s.readc:
			s.handleRead(ev)
		case n := <-s.prioNotifc.Receive():
			if !n.HasIdentifier() {
				n.SetIdentifier(s.nextId)
				s.nextId++
			}
			info("Sending notification %v", n.Identifier())
			s.doSend(n)
		case <-ticker:
			if s.conn != nil {
				s.conn.Expire()
			}
		}
	}

	close(s.donec)
}

func (s *Sender) handleRead(ev *readEvent) {
	var n *Notification
	var sent []*Notification
	conn := ev.conn

	conn.Close()
	if conn == s.conn {
		s.conn = nil
	}

	if resp := ev.resp; resp != nil {
		n = conn.GetSentNotification(resp.Identifier)

		if n == nil {
			info("Got a response for unknown notification %v", resp.Identifier)
		} else {
			info("Got a response for notification %v", resp.Identifier)

			// for ShutdownErrorStatus, the Identifier indicates the last
			// notification that was successfully sent
			if resp.Status != ShutdownErrorStatus {
				s.errorc <- &SenderError{
					Notification:  n,
					ErrorResponse: resp,
				}
			}
		}
	}

	if n != nil {
		sent = conn.GetSentNotificationsAfter(n.Identifier())
	} else {
		sent = conn.GetSentNotifications()
	}

	// requeue notifications before anything sent to s.notifc
	c := make(chan *Notification)
	s.prioNotifc.Add(c)

	go func() {
		for _, n := range sent {
			info("Requeuing notification %v", n.Identifier())
			c <- n
		}
		close(c)
	}()
}

func (s *Sender) doSend(n *Notification) {
	for {
		s.connect()

		if connError, err := s.conn.Write(n); err != nil {
			if connError {
				s.conn.Close()
				s.conn = nil
				info("%v; will retry", err)
			} else {
				info("%v; notification is lost", err)
				return
			}
		} else {
			break
		}
	}
}

func (s *Sender) connect() {
	for s.conn == nil {
		var conn conn
		var err error

		connect := func() error {
			info("Connecting to %v", s.addr)
			conn, err = s.newConn(s.addr, s.cert)
			if err != nil {
				info("Failed connecting to %v: %v; will retry", s.addr, err)
				return err
			}
			return nil
		}

		if backoff.Retry(connect, backoff.NewExponentialBackOff()) != nil {
			continue
		}

		info("Connected to %v", s.addr)

		go s.read(conn)

		s.conn = conn
	}
}

func (s *Sender) read(c conn) {
	for {
		select {
		case <-c.Done():
			return
		case pnr := <-c.Read():
			s.readc <- &readEvent{pnr, c}
		}
	}
}
