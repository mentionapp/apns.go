package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"

	_ "github.com/gsempe/apns/core"
	"github.com/streadway/amqp"
)

// MsgPushNotification is the message send from the cli to the apns standalone client
// It is also defined in the counterpart file (apns/apns/main.go apns/cli/main.go)
type MsgPushNotification struct {
	Text  string `json:"text"`
	Token string `json:"token"`
}

var (
	text  = flag.String("text", "Push notification text", "The text of the push notification")
	token = flag.String("token", "6f3031f2828aa1a369c78d3216be4b7c40ca7a8728a6a8d3e6229afc437b4ef1", "The token used to send the push notification")
)

func init() {
	flag.Parse()
}

func main() {

	pn := MsgPushNotification{Text: *text, Token: *token}
	body, err := json.Marshal(pn)
	if err != nil {
		log.Fatalf("Failed to encode the push notification: %s", err)
	}
	log.Println(body)
	send(body)
}

func send(body []byte) {

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
		"pushnotif", // name
		true,        // durable
		false,       // delete when usused
		false,       // exclusive
		false,       // no-wait
		nil,         // arguments
	)
	failOnError(err, "Failed to declare a queue")

	err = ch.Publish(
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        body,
		})
	failOnError(err, "Failed to publish a message")
}
