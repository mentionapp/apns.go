package apns

import "testing"

func TestAddShouldNotDeadlock(t *testing.T) {

	pc := newPriochan()

	c1 := make(chan *PushNotification)
	pc.Add(c1)

	// sending something on the channel, without a receiver
	c1 <- &PushNotification{}

	// now adding a new chan, shouldn't block
	c2 := make(chan *PushNotification)
	pc.Add(c2)
}
