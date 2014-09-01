package apns

import (
	"sync"
)

type pushNotificationQueue struct {
	mu    sync.RWMutex // Protection of queue accesses
	queue []*PushNotification
}

func NewQueue() pushNotificationQueue {
	return pushNotificationQueue{queue: []*PushNotification{}}
}

// head removes from the queue the push notification at the head of the queue
func (pnq *pushNotificationQueue) head() *PushNotification {

	pnq.mu.Lock()
	defer pnq.mu.Unlock()
	if len(pnq.queue) > 0 {
		pn := pnq.queue[0]
		copy(pnq.queue[0:], pnq.queue[1:])
		pnq.queue[len(pnq.queue)-1] = nil
		pnq.queue = pnq.queue[:len(pnq.queue)-1]
		return pn
	} else {
		return nil
	}
}

// pick removes from the queue the push notification at the head of the queue
func (pnq *pushNotificationQueue) pick() *PushNotification {

	pnq.mu.RLock()
	defer pnq.mu.RUnlock()
	if len(pnq.queue) > 0 {
		pn := pnq.queue[0]
		return pn
	} else {
		return nil
	}
}

// Remove removes the push notification with the id `pnIdentifier` if it exists in the list
func (pnq *pushNotificationQueue) Remove(pnIdentifier uint32) *PushNotification {

	pnq.mu.Lock()
	defer pnq.mu.Unlock()
	// Find the push notification with this identifier
	index := -1
	for i := 0; i < len(pnq.queue); i++ {
		if pnq.queue[i].Identifier == pnIdentifier {
			index = i
			break
		}
	}
	if index == -1 {
		return nil
	}
	pn := pnq.queue[index]
	copy(pnq.queue[index:], pnq.queue[index+1:])
	pnq.queue[len(pnq.queue)-1] = nil
	pnq.queue = pnq.queue[:len(pnq.queue)-1]
	return pn
}

// enqueue add the push notification at the end of the queue
func (pnq *pushNotificationQueue) enqueue(pn *PushNotification) {

	pnq.mu.Lock()
	defer pnq.mu.Unlock()
	pnq.queue = append(pnq.queue, pn)
}
