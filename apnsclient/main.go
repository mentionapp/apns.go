package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"

	"code.google.com/p/go.net/context"
	"github.com/gsempe/apns/core"
	"github.com/streadway/amqp"
)

// MsgPushNotification is the message send from the cli to the apns standalone client
// It is also defined in the counterpart file (apns/apns/main.go apns/cli/main.go)
type MsgPushNotification struct {
	Text  string `json:"text"`
	Token string `json:"token"`
}

var (
	sandbox  = flag.Bool("sandbox", false, "Use this flag to communicate with the sandbox and not the production")
	certFile = flag.String("cert", "apns-cert.pem", "The certificate file")
	keyFile  = flag.String("key", "apns-key.pem", "The key file")
)

func init() {
	flag.Parse()
}

func main() {

	var (
		gw  *apns.Gateway
		err error
	)
	conn, ch, msgs, err := initRabbitMQ()
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	defer ch.Close()

	ctx, _ := context.WithCancel(context.Background())
	if *sandbox {
		gw, err = apns.NewSandboxGateway(ctx, *certFile, *keyFile)
	} else {
		gw, err = apns.NewGateway(ctx, *certFile, *keyFile)
	}
	if err != nil {
		log.Fatalf("Failed to connect to gateway: %s", err)
		panic(err)
	}

	gw.Errors(func(pnr *apns.PushNotificationResponse) {
		log.Printf("Unable to send push notification with Id %d", pnr.Identifier)
	})

	runningIdentifier := uint32(0)

	for d := range msgs {
		msg := MsgPushNotification{}
		err := json.Unmarshal(d.Body, &msg)
		if err != nil {
			continue
		}
		runningIdentifier++

		payload := apns.NewPayload()
		payload.Alert = msg.Text
		payload.Badge = int(runningIdentifier)
		pn := apns.NewPushNotification()
		pn.DeviceToken = msg.Token
		pn.Identifier = runningIdentifier
		pn.AddPayload(payload)

		gw.Send(pn)
	}
}

// initRabbitMQ initialize the queue to communicate with the cli
func initRabbitMQ() (*amqp.Connection, *amqp.Channel, <-chan amqp.Delivery, error) {

	failOnError := func(err error, msg string) {
		if err != nil {
			log.Fatalf("%s: %s", msg, err)
			panic(fmt.Sprintf("%s: %s", msg, err))
		}
	}

	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")

	q, err := ch.QueueDeclare(
		"pushnotif", // name
		true,        // durable
		false,       // delete when usused
		false,       // exclusive
		false,       // no-wait
		nil,         // arguments
	)
	failOnError(err, "Failed to declare a queue")

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		true,   // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")

	if msgs == nil {
		log.Println("chan msgs is nil")
	}
	return conn, ch, msgs, err
}
