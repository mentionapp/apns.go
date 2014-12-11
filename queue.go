package apns

import (
	"container/list"
	"time"
)

type queue struct {
	l        *list.List
	m        map[NotificationIdentifier]*list.Element
	duration time.Duration
}

type queueElem struct {
	n       *Notification
	addedAt time.Time
}

// newQueue creates a new queue
func newQueue(duration time.Duration) *queue {
	return &queue{
		l:        list.New(),
		m:        make(map[NotificationIdentifier]*list.Element),
		duration: duration,
	}
}

func (q *queue) Add(n *Notification) {

	e := q.l.PushBack(&queueElem{n, time.Now()})
	q.m[n.Identifier()] = e
}

func (q *queue) Get(identifier NotificationIdentifier) *Notification {

	if e, ok := q.m[identifier]; ok {
		return e.Value.(*queueElem).n
	}

	return nil
}

func (q *queue) GetAllAfter(identifier NotificationIdentifier) []*Notification {

	var s []*Notification
	var e *list.Element
	var ok bool

	if e, ok = q.m[identifier]; ok {
		e = e.Next()
	} else {
		e = q.l.Front()
	}

	for ; e != nil; e = e.Next() {
		s = append(s, e.Value.(*queueElem).n)
	}

	return s
}

func (q *queue) GetAll() []*Notification {

	var s []*Notification

	for e := q.l.Front(); e != nil; e = e.Next() {
		s = append(s, e.Value.(*queueElem).n)
	}

	return s
}

func (q *queue) Expire() {

	now := time.Now()
	for {
		front := q.l.Front()
		if front == nil {
			break
		}
		elem := front.Value.(*queueElem)
		if now.Sub(elem.addedAt) > q.duration {
			q.l.Remove(front)
			delete(q.m, elem.n.Identifier())
		} else {
			break
		}
	}
}
