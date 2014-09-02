package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

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

func createPushNotification(identifier uint32, text, deviceToken string) *apns.PushNotification {

	payload := apns.NewPayload()
	payload.Alert = text
	payload.Badge = 42
	payload.Sound = "bingbong.aiff"

	pn := apns.NewPushNotification()
	pn.DeviceToken = deviceToken
	pn.Identifier = identifier
	pn.AddPayload(payload)
	return pn
}

var (
	certFile = flag.String("cert", "apns-cert.pem", "The certificate file")
	keyFile  = flag.String("key", "apns-key.pem", "The key file")
)

func init() {
	flag.Parse()
}

func main() {

	conn, ch, msgs, err := initRabbitMQ()
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	defer ch.Close()

	ctx, _ := context.WithCancel(context.Background())
	gw, err := apns.NewGateway(ctx, *certFile, *keyFile)
	if err != nil {
		log.Fatalf("Failed to connect to gateway: %s", err)
		panic(err)
	}

	runningId := uint32(0)

	for d := range msgs {
		pn := MsgPushNotification{}
		err := json.Unmarshal(d.Body, &pn)
		if err != nil {
			continue
		}
		runningId++
		gw.Send(createPushNotification(runningId, pn.Text, pn.Token))
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
		"hello", // name
		true,    // durable
		false,   // delete when usused
		false,   // exclusive
		false,   // no-wait
		nil,     // arguments
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
