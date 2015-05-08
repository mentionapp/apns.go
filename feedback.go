package apns

import (
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"io"
	"strings"
	"time"

	"code.google.com/p/go.net/context"
)

const (
	FeedbackGateway        string = "feedback.push.apple.com:2196"
	FeedbackSandboxGateway string = "feedback.sandbox.push.apple.com:2196"
)

const (
	feedbackCheckPeriod time.Duration = 5 * time.Second
)

type FeedbackMessage struct {
	Unsubscribe time.Time
	DeviceToken string
}

// Feedback only read binary apple socket
type Feedback struct {
	addr     string
	cert     *tls.Certificate
	messages chan *FeedbackMessage
}

func NewFeedback(ctx context.Context, addr string, cert *tls.Certificate) (f *Feedback) {
	f = &Feedback{
		addr:     addr,
		cert:     cert,
		messages: make(chan *FeedbackMessage),
	}
	go f.reader(ctx)
	return
}

func (f *Feedback) Messages() <-chan *FeedbackMessage {
	return f.messages
}

func (f *Feedback) reader(ctx context.Context) {
	var stop bool
	go func() {
		for {
			select {
			case <-ctx.Done():
				stop = true
				return
			}
		}
	}()
	for {
		if stop {
			break
		}
		result, err := f.receive()
		if err != nil && err != io.EOF {
			info("Feedback receive err: %v", err)
		} else {
			for _, msg := range result {
				info("Feedback receive msg: %v", msg)
				f.messages <- msg
			}
		}
		time.Sleep(feedbackCheckPeriod)
	}
}

func (f *Feedback) receive() (result []*FeedbackMessage, err error) {
	info("Connecting to %v", f.addr)
	conn, err := newTlsConn(f.addr, f.cert)
	if err != nil {
		info("Failed connecting to %v: %v; will retry", f.addr, err)
		return
	}
	info("Connected to %v", f.addr)
	defer conn.Close()

	result = make([]*FeedbackMessage, 0, 1)
	for {
		var unsubTime uint32
		var tokenLen uint16
		if err = binary.Read(conn, binary.BigEndian, &unsubTime); err != nil {
			return result, err
		}
		if err = binary.Read(conn, binary.BigEndian, &tokenLen); err != nil {
			return result, err
		}
		bToken := make([]byte, int(tokenLen))
		n, err := io.ReadFull(conn, bToken)
		if err != nil {
			return result, err
		}
		if n != int(tokenLen) {
			return result, err
		}

		token := hex.EncodeToString(bToken)
		token = strings.ToLower(token)

		result = append(result, &FeedbackMessage{
			Unsubscribe: time.Unix(int64(unsubTime), 0),
			DeviceToken: token,
		})
	}

}
