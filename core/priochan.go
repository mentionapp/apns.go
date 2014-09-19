package apns

type priochan struct {
	chanc chan chan *PushNotification
	outc  chan *PushNotification
}

func newPriochan() *priochan {
	m := &priochan{
		chanc: make(chan chan *PushNotification),
		outc:  make(chan *PushNotification),
	}
	go m.read()
	return m
}

func (m *priochan) Add(c chan *PushNotification) {
	m.chanc <- c
}

func (m *priochan) Receive() <-chan *PushNotification {
	return m.outc
}

func (m *priochan) Close() {
	close(m.chanc)
}

func (m *priochan) read() {
	var stack []chan *PushNotification
	var current chan *PushNotification

	handleChan := func(c chan *PushNotification, ok bool) bool {
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
