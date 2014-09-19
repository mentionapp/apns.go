package apns

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
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

type Payload map[string]interface{}

type NotificationIdentifier uint32

type NotificationPriority uint8

type Notification struct {
	deviceToken string
	payload     Payload
	identifier  NotificationIdentifier
	expiry      time.Duration
	priority    NotificationPriority
}

// AlertDictionary is a localized alert text
type AlertDictionary struct {
	Body         string   `json:"body,omitempty"`
	ActionLocKey string   `json:"action-loc-key,omitempty"`
	LocKey       string   `json:"loc-key,omitempty"`
	LocArgs      []string `json:"loc-args,omitempty"`
	LaunchImage  string   `json:"launch-image,omitempty"`
}

func (p Payload) SetAlertString(alert string) {
	p.aps()["alert"] = alert
}

func (p Payload) SetAlertDictionary(alert *AlertDictionary) {
	p.aps()["alert"] = alert
}

func (p Payload) SetBadge(badge int) {
	p.aps()["badge"] = badge
}

func (p Payload) SetSound(sound string) {
	p.aps()["sound"] = sound
}

func (p Payload) Set(name string, value interface{}) {
	p[name] = value
}

func (p Payload) ToJSON() ([]byte, error) {
	return json.Marshal(p)
}

func (p Payload) aps() map[string]interface{} {
	if e, ok := p["aps"]; ok {
		if aps, ok := e.(map[string]interface{}); ok {
			return aps
		}
	}
	aps := make(map[string]interface{})
	p["aps"] = aps
	return aps
}

func NewNotification() *Notification {
	n := &Notification{}
	n.payload = make(Payload)
	n.priority = 10
	return n
}

func (n *Notification) SetDeviceToken(token string) {
	n.deviceToken = token
}

func (n *Notification) DeviceToken() string {
	return n.deviceToken
}

func (n *Notification) Payload() Payload {
	return n.payload
}

func (n *Notification) SetIdentifier(identifier NotificationIdentifier) {
	n.identifier = identifier
}

func (n *Notification) Identifier() NotificationIdentifier {
	return n.identifier
}

func (n *Notification) SetExpiry(expiry time.Duration) {
	n.expiry = expiry
}

func (n *Notification) Expiry() time.Duration {
	return n.expiry
}

func (n *Notification) SetPriority(priority NotificationPriority) {
	n.priority = priority
}

func (n *Notification) Priority() NotificationPriority {
	return n.priority
}

// NewAlertDictionary creates a new AlertDictionary
func NewAlertDictionary() *AlertDictionary {
	return &AlertDictionary{}
}

// Encode encodes a notification packet
func (n *Notification) Encode() ([]byte, error) {

	token, err := hex.DecodeString(n.deviceToken)
	if err != nil {
		return nil, fmt.Errorf("failed decoding device token %q: %v", n.deviceToken, err)
	}

	payload, err := n.payload.ToJSON()
	if err != nil {
		return nil, err
	}

	if len(payload) > MaxPayloadSizeBytes {
		return nil, fmt.Errorf("Payload is larger than the %v byte limit", MaxPayloadSizeBytes)
	}

	BE := binary.BigEndian

	frameBuffer := &bytes.Buffer{}

	binary.Write(frameBuffer, BE, uint8(deviceTokenItemid))
	binary.Write(frameBuffer, BE, uint16(deviceTokenLength))
	binary.Write(frameBuffer, BE, token)

	binary.Write(frameBuffer, BE, uint8(payloadItemid))
	binary.Write(frameBuffer, BE, uint16(len(payload)))
	binary.Write(frameBuffer, BE, payload)

	binary.Write(frameBuffer, BE, uint8(notificationIdentifierItemid))
	binary.Write(frameBuffer, BE, uint16(notificationIdentifierLength))
	binary.Write(frameBuffer, BE, n.identifier)

	binary.Write(frameBuffer, BE, uint8(expirationDateItemid))
	binary.Write(frameBuffer, BE, uint16(expirationDateLength))
	binary.Write(frameBuffer, BE, uint32(n.expiry.Seconds()))

	binary.Write(frameBuffer, BE, uint8(priorityItemid))
	binary.Write(frameBuffer, BE, uint16(priorityLength))
	binary.Write(frameBuffer, BE, n.priority)

	buffer := bytes.NewBuffer([]byte{})
	binary.Write(buffer, BE, uint8(pushCommandValue))
	binary.Write(buffer, BE, uint32(frameBuffer.Len()))
	binary.Write(buffer, BE, frameBuffer.Bytes())

	return buffer.Bytes(), nil
}
