# apns

Client library for Apple Push Notification, focusing on reliability.

## Example usage

``` go
package main

import (
	"crypto/tls"

	"code.google.com/p/go.net/context"
	"github.com/mentionapp/apns.go"
)

func main() {

	cert, err := tls.LoadX509KeyPair("cert.crt", "cert.key")
	if err != nil {
		panic(err)
	}
	
	apns.Verbose = true

	sender := apns.NewSender(context.TODO(), apns.SenderSandboxGateway, &cert)

	go func() {
		// sender.Errors() is a channel
		// It receives permanent sending errors (e.g. the token is invalid).
		// For this kind of errors, the library doesn't retry. The application
		// might want to take action, though.
		for senderError := range sender.Errors() {
			switch senderError.ErrorResponse.Status {
			case apns.InvalidTokenSizeErrorStatus, apns.InvalidTokenErrorStatus:
				// We tried to send a message to an invalid token
			}
		}
	}()

	// msgs is an imaginary channel with messages to send
	for msg := range msgs {
		notif := apns.NewNotification()
		notif.SetDeviceToken(msg.Token)
		payload := &apns.Payload{}
		payload.SetAlertString(msg.Text)
		notif.SetPayload(payload)
		sender.Notifications() <- notif
	}
}
```

## Credits

 - [gsempe](https://github.com/gsempe)
 - [arnaud-lb](https://github.com/arnaud-lb)
 - [Contributors](https://github.com/mentionapp/apns.go/graphs/contributors)
 - Originally based on [anachronistic/apns](https://github.com/anachronistic/apns)

