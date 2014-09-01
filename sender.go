package apns

import (
	"log"
	"time"

	"code.google.com/p/go.net/context"
)

// Sender is informations needed to send push notifications to the APNs
type Sender struct {
	client       *PersistentClient              // The client that sends push notifications to the APNs
	waitingQueue pushNotificationQueue          // Queue of push notification waiting for to be sent
	sendingQueue pushNotificationQueue          // Queue of push notification currently in the sending state
	pnc          chan *PushNotification         // Push Notification Channel
	pnrc         chan *PushNotificationResponse // Push Notification Response Channel
	pnrec        chan *PushNotificationResponse // Push Notification Response Error Channel: the channel to send error back to the `gateway`
}

// NewSender creates a sender.
// It Manages the connection with the APNs, the waitingQueue of push notifications:
// - requeue the push notifications with a temporary error
// - report errors to the caller
func NewSender(ctx context.Context, gateway, ip, certificateFile, keyFile string) (*Sender, error) {

	c, err := NewPersistentClient(gateway, ip, certificateFile, keyFile)
	if err != nil {
		return nil, err
	}
	wq := NewQueue()
	sq := NewQueue()
	pnc := make(chan *PushNotification)
	pnrc := make(chan *PushNotificationResponse)
	s := &Sender{client: c, waitingQueue: wq, sendingQueue: sq, pnc: pnc, pnrc: pnrc}

	go s.senderJob(ctx) // Launch the job: come on baby!
	return s, nil
}

// Send enqueues a new push notification to the sender and initiate the sending of all the push notification already queued
func (s *Sender) Send(pn *PushNotification) {

	s.waitingQueue.enqueue(pn)
	pnhead := s.waitingQueue.pick()
	if pnhead != nil {
		s.pnc <- pnhead
	}
}

// senderJob does the loop connection, send push notifications, manage errors
func (s *Sender) senderJob(ctx context.Context) {

	cnxc := make(chan struct{}) // Chan to manage re-connections
	for {
		err := s.client.Connect() // Reconnect to APNs if needed
		if err != nil {
			log.Println("Connection to APNs error ", err)
			// Connection failed so we have to retry it later
			go func() {
				// TODO GSE : Improve the reconnection managment
				time.Sleep(time.Second * 5)
				cnxc <- struct{}{}
			}()
		}
		select {
		case <-ctx.Done():
			s.client.Close()
			return
		case <-cnxc:
			// Let the for loop retry the connexion
		case <-s.pnc:
			for {
				pn := s.waitingQueue.head()
				if pn == nil {
					break
				}
				s.sendingQueue.enqueue(pn)
				go func() {
					resp := s.client.Send(ctx, pn)
					s.pnrc <- resp
				}()
			}

		case pnr := <-s.pnrc:
			// The push notification response is not identifiable: Abort
			// It's a side effect of an error on another push notification and it is managed
			// by requeueing in `workingQueue` the push notifications behind the push notification with an error
			if pnr.Identifier == 0 {
				continue
			}
			// A new response is arrived
			// - it's a success: Forget about this push notification
			// - it's an APNs error or another local error (not retry): Gives the information to the client and requeue the push notifications behind this one
			if pnr.Success == true {
				// TODO GSE: Here, should we remove all the push notification ahead this one?
				s.sendingQueue.Remove(pnr.Identifier)
			} else {
				log.Println("Push notification with ID ", pnr.Identifier, " has an error ", pnr.Error)
				s.client.Close()
				var pnhead *PushNotification
				for {
					pnhead = s.sendingQueue.head()
					if pnhead == nil {
						break
					}
					if pnhead.Identifier == pnr.Identifier {
						break
					}
				}
				if pnhead != nil {
					if pnr.ResponseCommand == LocalResponseCommand && pnr.ResponseStatus == RetryPushNotificationStatus { // retry to send the push notification
						log.Println("Requeue the Push notification with ID ", pnhead.Identifier)
						go s.Send(pnhead)
					} else {
						if s.pnrec != nil {
							go func() {
								select {
								case <-ctx.Done(): // In case of the race condition: `s.pnrec` was not nil and it is now nil: It must be because the gateway has been canceled
								case s.pnrec <- pnr:
								}
							}()
						}
					}
					for {
						pn := s.sendingQueue.head()
						if pn == nil {
							break
						} else {
							log.Println("reschedule push notification with ID ", pn.Identifier)
							go s.Send(pn)
						}
					}
				}

			}
		}
	}
	return
}
