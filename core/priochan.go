package apns

type priochan struct {
	chanc chan chan *Notification
	outc  chan *Notification
}

func newPriochan() *priochan {
	m := &priochan{
		chanc: make(chan chan *Notification),
		outc:  make(chan *Notification),
	}
	go m.read()
	return m
}

func (m *priochan) Add(c chan *Notification) {
	m.chanc <- c
}

func (m *priochan) Receive() <-chan *Notification {
	return m.outc
}

func (m *priochan) Close() {
	close(m.chanc)
}

func (m *priochan) read() {
	var stack []chan *Notification
	var current chan *Notification
	var send func(e *Notification) bool

	handleChan := func(c chan *Notification, ok bool) (stillOpened bool) {
		if !ok {
			return false
		}
		if current != nil {
			stack = append(stack, current)
		}
		current = c
		return true
	}

	receive := func() {
		for {
			select {
			case e, ok := <-current:
				if ok {
					if !send(e) {
						return
					}
				} else {
					if len(stack) > 0 {
						current = stack[len(stack)-1]
						stack = stack[0 : len(stack)-1]
					} else {
						current = nil
					}
				}
			case c, ok := <-m.chanc:
				if !handleChan(c, ok) {
					return
				}
			}
		}
	}

	send = func(e *Notification) (stillOpened bool) {
		for {
			select {
			case m.outc <- e:
				return true
			case c, ok := <-m.chanc:
				if !handleChan(c, ok) {
					return false
				}
			}
		}
	}

	receive()
}
