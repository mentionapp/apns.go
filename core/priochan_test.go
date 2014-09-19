package apns

import (
	"sync"
	"testing"
)

func TestPriochan(t *testing.T) {

	notif := func(id int) *Notification {
		n := &Notification{}
		n.SetIdentifier(NotificationIdentifier(id))
		return n
	}

	pc := newPriochan()

	c1 := make(chan *Notification)
	pc.Add(c1)

	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {

		last := NotificationIdentifier(0)

		// We test that the Receive() chan is closed at some point
		for n := range pc.Receive() {

			t.Logf("Received elem %v", n.Identifier())

			// We test that elements are received in expected order
			if n.Identifier() < last {
				t.Fatalf("Elem %v is out of order", n.Identifier())
			}

			last = n.Identifier()

			if 3 == n.Identifier() {
				c2 := make(chan *Notification)
				pc.Add(c2)
				go func() {
					c2 <- notif(4)
					c2 <- notif(5)
					c2 <- notif(6)
					close(c2)
				}()
			}
		}
		wg.Done()
	}()

	go func() {

		c1 <- notif(1)
		c1 <- notif(2)
		c1 <- notif(3)

		c1 <- notif(7)
		c1 <- notif(8)
		c1 <- notif(9)

		close(c1)
		pc.Close()
	}()

	wg.Wait()
}

func TestAddShouldNotDeadlock(t *testing.T) {

	pc := newPriochan()

	c1 := make(chan *Notification)
	pc.Add(c1)

	// sending something on the channel, without a receiver
	c1 <- &Notification{}

	// now adding a new chan, shouldn't block
	c2 := make(chan *Notification)
	pc.Add(c2)
}
