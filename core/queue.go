package apns

import (
	"container/list"
	"time"
)

type queue struct {
	l        *list.List
	m        map[uint32]*list.Element
	duration time.Duration
}

type queueElem struct {
	pn      *PushNotification
	addedAt time.Time
}

// newQueue creates a new queue
func newQueue(duration time.Duration) *queue {
	return &queue{
		l:        list.New(),
		m:        make(map[uint32]*list.Element),
		duration: duration,
	}
}

func (q *queue) Add(pn *PushNotification) {

	e := q.l.PushBack(&queueElem{pn, time.Now()})
	q.m[pn.Identifier] = e
}

func (q *queue) Get(identifier uint32) *PushNotification {

	if e, ok := q.m[identifier]; ok {
		return e.Value.(*queueElem).pn
	}

	return nil
}

func (q *queue) GetAllAfter(identifier uint32) []*PushNotification {

	var s []*PushNotification
	var e *list.Element
	var ok bool

	if e, ok = q.m[identifier]; ok {
		e = e.Next()
	} else {
		e = q.l.Front()
	}

	for ; e != nil; e = e.Next() {
		s = append(s, e.Value.(*queueElem).pn)
	}

	return s
}

func (q *queue) GetAll() []*PushNotification {

	var s []*PushNotification

	for e := q.l.Front(); e != nil; e = e.Next() {
		s = append(s, e.Value.(*queueElem).pn)
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
			delete(q.m, elem.pn.Identifier)
		} else {
			break
		}
	}
}
