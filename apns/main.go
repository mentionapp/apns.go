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

	msgs, err := initRabbitMQ()
	if err != nil {
		panic(err)
	}

	ctx, _ := context.WithCancel(context.Background())
	gw, err := apns.NewGateway(ctx, *certFile, *keyFile)
	if err != nil {
		log.Fatalf("Failed to connect to gateway: %s", err)
		panic(err)
	}

	runningId := uint32(0)

	// for {
	// 	select {
	// 	case d := <-msgs:
	// 		log.Printf("Received a message: %s", d.Body)
	// 		pn := MsgPushNotification{}
	// 		err := json.Unmarshal(d.Body, &pn)
	// 		if err != nil {
	// 			log.Printf("Failed to decode msg %v", err)
	// 			time.Sleep(time.Second * 3)
	// 			continue
	// 		}
	// 		runningId++
	// 		log.Printf("send push notif with text `%s` to token: `%s`", pn.Text, pn.Token)
	// 		gw.Send(createPushNotification(runningId, pn.Text, pn.Token))
	// 	}
	// }

	for d := range msgs {
		log.Printf("Received a message: %s", d.Body)
		pn := MsgPushNotification{}
		err := json.Unmarshal(d.Body, &pn)
		if err != nil {
			log.Printf("Failed to decode msg %v", err)
			time.Sleep(time.Second * 3)
			continue
		}
		runningId++
		log.Printf("send push notif with text `%s` to token: `%s`", pn.Text, pn.Token)
		gw.Send(createPushNotification(runningId, pn.Text, pn.Token))
	}
}

// initRabbitMQ initialize the queue to communicate with the cli
func initRabbitMQ() (<-chan amqp.Delivery, error) {

	failOnError := func(err error, msg string) {
		if err != nil {
			log.Fatalf("%s: %s", msg, err)
			panic(fmt.Sprintf("%s: %s", msg, err))
		}
	}

	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

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
	return msgs, err
}
