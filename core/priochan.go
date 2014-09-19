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

	handleChan := func(c chan *Notification, ok bool) bool {
		if !ok {
			return false
		}
		if current != nil {
			stack = append(stack, current)
		}
		current = c
		return true
	}

	for {
		select {
		case e, ok := <-current:
			if ok {
				sent := false
				for !sent {
					select {
					case m.outc <- e:
						sent = true
					case c, ok := <-m.chanc:
						if !handleChan(c, ok) {
							return
						}
					}
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
