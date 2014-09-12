package apns

import (
	"container/list"
	"sync"
	"time"
)

type queue struct {
	sync.Mutex
	l        *list.List
	m        map[uint32]*list.Element
	duration time.Duration
}

type queueElem struct {
	pn      *PushNotification
	addedAt time.Time
}

// NewQueue creates a new queue
func NewQueue(duration time.Duration) *queue {
	return &queue{
		l: list.New(),
		m: make(map[uint32]*list.Element),
	}
}

func (q *queue) Add(pn *PushNotification) {
	q.Lock()
	defer q.Unlock()

	e := q.l.PushBack(&queueElem{pn, time.Now()})
	q.m[pn.Identifier] = e
}

func (q *queue) Expire() {
	q.Lock()
	defer q.Unlock()

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

func (q *queue) Remove(identifier uint32) *PushNotification {
	q.Lock()
	defer q.Unlock()

	if e, ok := q.m[identifier]; ok {
		q.l.Remove(e)
		delete(q.m, identifier)
		return e.Value.(*queueElem).pn
	}

	return nil
}

func (q *queue) RemoveAll() {
	q.l = list.New()
	q.m = make(map[uint32]*list.Element)
}

func (q *queue) GetAllAfter(identifier uint32) []*PushNotification {
	q.Lock()
	defer q.Unlock()

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
	q.Lock()
	defer q.Unlock()

	var s []*PushNotification

	for e := q.l.Front(); e != nil; e = e.Next() {
		s = append(s, e.Value.(*queueElem).pn)
	}

	return s
}
