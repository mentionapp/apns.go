package apns

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
)

// Push commands always start with command value 2.
const pushCommandValue = 2

// Your total notification payload cannot exceed 256 bytes.
const MaxPayloadSizeBytes = 256

// Constants related to the payload fields and their lengths.
const (
	deviceTokenItemid            = 1
	payloadItemid                = 2
	notificationIdentifierItemid = 3
	expirationDateItemid         = 4
	priorityItemid               = 5
	deviceTokenLength            = 32
	notificationIdentifierLength = 4
	expirationDateLength         = 4
	priorityLength               = 1
)

// Payload contains the notification data for your request.
type Payload map[string]interface{}

// NewPayload creates a new Payload
func NewPayload() Payload {
	return make(Payload)
}

func (p Payload) SetAlertString(alert string) {
	p["alert"] = alert
}

func (p Payload) SetAlertDictionary(alert AlertDictionary) {
	p["alert"] = alert
}

func (p Payload) SetBadge(badge int) {
	p["badge"] = badge
}

func (p Payload) SetSound(sound string) {
	p["sound"] = sound
}

// AlertDictionary is a more complex notification payload.
//
// From the APN docs: "Use the ... alert dictionary in general only if you absolutely need to."
// The AlertDictionary is suitable for specific localization needs.
type AlertDictionary struct {
	Body         string   `json:"body,omitempty"`
	ActionLocKey string   `json:"action-loc-key,omitempty"`
	LocKey       string   `json:"loc-key,omitempty"`
	LocArgs      []string `json:"loc-args,omitempty"`
	LaunchImage  string   `json:"launch-image,omitempty"`
}

// NewAlertDictionary creates and returns an AlertDictionary structure.
func NewAlertDictionary() *AlertDictionary {
	return new(AlertDictionary)
}

// PushNotification is the wrapper for the Payload.
// The length fields are computed in ToBytes() and aren't represented here.
type PushNotification struct {
	Identifier  uint32
	Expiry      uint32
	DeviceToken string
	payload     map[string]interface{}
	Priority    uint8
}

// NewPushNotification creates and returns a PushNotification structure.
// It also initializes the pseudo-random identifier.
func NewPushNotification() (pn *PushNotification) {
	pn = new(PushNotification)
	pn.payload = make(map[string]interface{})
	pn.Identifier = rand.Uint32()
	pn.Priority = 10
	return
}

// AddPayload sets the "aps" payload section of the request.
func (pn *PushNotification) SetPayload(p Payload) {
	pn.Set("aps", p)
}

// Get returns the value of a payload key, if it exists.
func (pn *PushNotification) Get(key string) interface{} {
	return pn.payload[key]
}

// Set defines the value of a payload key.
func (pn *PushNotification) Set(key string, value interface{}) {
	pn.payload[key] = value
}

// PayloadJSON returns the current payload in JSON format.
func (pn *PushNotification) PayloadJSON() ([]byte, error) {
	return json.Marshal(pn.payload)
}

// PayloadString returns the current payload in string format.
func (pn *PushNotification) PayloadString() (string, error) {
	j, err := pn.PayloadJSON()
	return string(j), err
}

// ToBytes returns a byte array of the complete PushNotification
// struct. This array is what should be transmitted to the APN Service.
func (pn *PushNotification) ToBytes() ([]byte, error) {
	token, err := hex.DecodeString(pn.DeviceToken)
	if err != nil {
		return nil, fmt.Errorf("failed decoding device token %q: %v", pn.DeviceToken, err)
	}
	payload, err := pn.PayloadJSON()
	if err != nil {
		return nil, err
	}
	if len(payload) > MaxPayloadSizeBytes {
		return nil, errors.New("payload is larger than the " + strconv.Itoa(MaxPayloadSizeBytes) + " byte limit")
	}

	frameBuffer := new(bytes.Buffer)
	binary.Write(frameBuffer, binary.BigEndian, uint8(deviceTokenItemid))
	binary.Write(frameBuffer, binary.BigEndian, uint16(deviceTokenLength))
	binary.Write(frameBuffer, binary.BigEndian, token)
	binary.Write(frameBuffer, binary.BigEndian, uint8(payloadItemid))
	binary.Write(frameBuffer, binary.BigEndian, uint16(len(payload)))
	binary.Write(frameBuffer, binary.BigEndian, payload)
	binary.Write(frameBuffer, binary.BigEndian, uint8(notificationIdentifierItemid))
	binary.Write(frameBuffer, binary.BigEndian, uint16(notificationIdentifierLength))
	binary.Write(frameBuffer, binary.BigEndian, pn.Identifier)
	binary.Write(frameBuffer, binary.BigEndian, uint8(expirationDateItemid))
	binary.Write(frameBuffer, binary.BigEndian, uint16(expirationDateLength))
	binary.Write(frameBuffer, binary.BigEndian, pn.Expiry)
	binary.Write(frameBuffer, binary.BigEndian, uint8(priorityItemid))
	binary.Write(frameBuffer, binary.BigEndian, uint16(priorityLength))
	binary.Write(frameBuffer, binary.BigEndian, pn.Priority)

	buffer := bytes.NewBuffer([]byte{})
	binary.Write(buffer, binary.BigEndian, uint8(pushCommandValue))
	binary.Write(buffer, binary.BigEndian, uint32(frameBuffer.Len()))
	binary.Write(buffer, binary.BigEndian, frameBuffer.Bytes())
	return buffer.Bytes(), nil
}
