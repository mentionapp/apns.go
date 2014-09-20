package apns

import (
	"crypto/tls"
	"errors"
	"log"
	"testing"
	"time"

	"code.google.com/p/go.net/context"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockConn struct {
	mock.Mock
	readc chan *ErrorResponse
	donec chan struct{}
	sent  *queue
	write func(n *Notification) (bool, error)
}

func newMockConn() *mockConn {
	m := &mockConn{}
	m.readc = make(chan *ErrorResponse)
	m.donec = make(chan struct{})
	m.sent = newQueue(time.Second * 300)
	return m
}

func (c *mockConn) Write(n *Notification) (connError bool, err error) {
	if c.write != nil {
		connError, err = c.write(n)
	}
	if err == nil {
		c.sent.Add(n)
	}
	return
}

func (c *mockConn) Read() <-chan *ErrorResponse {
	return c.readc
}

func (c *mockConn) Done() <-chan struct{} {
	return c.donec
}

func (c *mockConn) Close() {
	c.Mock.Called()
}

func (c *mockConn) GetSentNotification(identifier NotificationIdentifier) *Notification {
	return c.sent.Get(identifier)
}

func (c *mockConn) GetSentNotificationsAfter(identifier NotificationIdentifier) []*Notification {
	return c.sent.GetAllAfter(identifier)
}

func (c *mockConn) GetSentNotifications() []*Notification {
	return c.sent.GetAll()
}

func (c *mockConn) Expire() {
}

func createNotifs(num int) []*Notification {

	n := []*Notification{}

	for i := 0; i < num; i++ {
		t := NewNotification()
		t.SetIdentifier(NotificationIdentifier(i))
		n = append(n, t)
	}

	return n
}

func sendNotifs(s *Sender, n []*Notification) {
	for _, t := range n {
		s.Notifications() <- t
	}
}

func drainErrors(s *Sender) {
	for {
		select {
		case <-s.ErrorFeedbacks():
		case <-s.Done():
			return
		}
	}
}

func waitUntil(f func() bool) {
	for start := time.Now(); time.Now().Sub(start) < time.Second*5; {
		if f() {
			return
		}
		<-time.After(time.Millisecond * 100)
	}
}

func TestSenderRetriesOnErrorResponse(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	addr := "example.com:1234"
	cert := &tls.Certificate{}

	n := createNotifs(6)
	mocks := []*mock.Mock{}
	conns := 0
	sent := []NotificationIdentifier{}

	s := NewSender(ctx, addr, cert)
	s.newConn = func(addr string, cert *tls.Certificate) (conn, error) {

		var c *mockConn

		switch conns {
		case 0:

			c = newMockConn()

			c.write = func(n *Notification) (connError bool, err error) {
				if n.Identifier() < 1 {
					sent = append(sent, n.Identifier())
				}
				if n.Identifier() == 4 {
					go func() {
						c.readc <- &ErrorResponse{Identifier: 1}
						log.Println("Sent the error response")
					}()
				}
				return false, nil
			}

			c.On("Close").Return().Once()

		case 1:

			c = newMockConn()

			c.write = func(n *Notification) (connError bool, err error) {
				sent = append(sent, n.Identifier())
				return
			}

			c.On("Close").Return().Once()
		}

		conns += 1

		mocks = append(mocks, &c.Mock)
		return c, nil
	}

	go sendNotifs(s, n)
	go drainErrors(s)

	waitUntil(func() bool { return len(sent) == 5 })

	cancel()

	<-s.Done()

	for _, m := range mocks {
		m.AssertExpectations(t)
	}

	assert.Equal(t, []NotificationIdentifier{0, 2, 3, 4, 5}, sent)
}

func TestSenderRetriesOnWriteErrors(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	addr := "example.com:1234"
	cert := &tls.Certificate{}

	n := createNotifs(6)
	mocks := []*mock.Mock{}
	conns := 0
	sent := []NotificationIdentifier{}

	s := NewSender(ctx, addr, cert)
	s.newConn = func(addr string, cert *tls.Certificate) (conn, error) {

		var c *mockConn

		switch conns {
		case 0:

			c = newMockConn()

			c.write = func(n *Notification) (connError bool, err error) {
				if n.Identifier() == 4 {
					return true, errors.New("some error")
				}
				sent = append(sent, n.Identifier())
				return
			}

			c.On("Close").Return().Once()

		case 1:

			c = newMockConn()

			c.write = func(n *Notification) (connError bool, err error) {
				sent = append(sent, n.Identifier())
				return
			}

			c.On("Close").Return().Once()
		}

		conns += 1

		mocks = append(mocks, &c.Mock)
		return c, nil
	}

	go sendNotifs(s, n)
	go drainErrors(s)

	waitUntil(func() bool { return len(sent) == 6 })

	cancel()

	<-s.Done()

	for _, m := range mocks {
		m.AssertExpectations(t)
	}

	assert.Equal(t, []NotificationIdentifier{0, 1, 2, 3, 4, 5}, sent)
}

func TestSenderDoesNotRetryNonConnErrors(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	addr := "example.com:1234"
	cert := &tls.Certificate{}

	n := createNotifs(6)
	mocks := []*mock.Mock{}
	conns := 0
	sent := []NotificationIdentifier{}

	s := NewSender(ctx, addr, cert)
	s.newConn = func(addr string, cert *tls.Certificate) (conn, error) {

		var c *mockConn

		switch conns {
		case 0:

			c = newMockConn()

			c.write = func(n *Notification) (connError bool, err error) {
				if n.Identifier() == 4 {
					return false, errors.New("some error")
				}
				sent = append(sent, n.Identifier())
				return
			}

			c.On("Close").Return().Once()

		case 1:

			c = newMockConn()

			c.write = func(n *Notification) (connError bool, err error) {
				sent = append(sent, n.Identifier())
				return
			}

			c.On("Close").Return().Once()
		}

		conns += 1

		mocks = append(mocks, &c.Mock)
		return c, nil
	}

	go sendNotifs(s, n)
	go drainErrors(s)

	waitUntil(func() bool { return len(sent) == 5 })

	cancel()

	<-s.Done()

	for _, m := range mocks {
		m.AssertExpectations(t)
	}

	assert.Equal(t, []NotificationIdentifier{0, 1, 2, 3, 5}, sent)
}
