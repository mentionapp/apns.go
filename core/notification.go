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

// Payload represents a notification payload
type Payload map[string]interface{}

// NotificationIdentifier represents a notification identifier
type NotificationIdentifier uint32

// NotificationPriority represents a notification priority
type NotificationPriority uint8

// Notification represents a notification
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

const (
	// ImmediatePriority sets the push message to be sent immediately
	ImmediatePriority NotificationPriority = 10
	// PowerSavingPriority sets the push message to be sent at a time that conserves power on the device receiving it
	PowerSavingPriority NotificationPriority = 5
)

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

// SetAlertString sets the alert item as a string
func (p Payload) SetAlertString(alert string) {
	p.aps()["alert"] = alert
}

// SetAlertDictionary sets the alert item as a dictionary
func (p Payload) SetAlertDictionary(alert *AlertDictionary) {
	p.aps()["alert"] = alert
}

// SetBadge sets the badge item
func (p Payload) SetBadge(badge int) {
	p.aps()["badge"] = badge
}

// SetSound sets the sound item
func (p Payload) SetSound(sound string) {
	p.aps()["sound"] = sound
}

// Set sets a custom item outside the aps namespace
func (p Payload) Set(name string, value interface{}) {
	p[name] = value
}

// ToJSON encodes the Payload to JSON. The encoded payload cannot exceed
// MaxPayloadSizeBytes bytes
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

// NewNotification creates a new Notification
func NewNotification() *Notification {
	n := &Notification{}
	n.payload = make(Payload)
	n.priority = ImmediatePriority
	return n
}

// SetDeviceToken sets the device token. Must be a 64 bytes hex string.
func (n *Notification) SetDeviceToken(token string) {
	n.deviceToken = token
}

// DeviceToken returns the device token
func (n *Notification) DeviceToken() string {
	return n.deviceToken
}

// Payload returns the Payload
func (n *Notification) Payload() Payload {
	return n.payload
}

// SetIdentifier returns the Identifier. Must be unique among a Sender instance
func (n *Notification) SetIdentifier(identifier NotificationIdentifier) {
	n.identifier = identifier
}

// Identifier returns the Identifier
func (n *Notification) Identifier() NotificationIdentifier {
	return n.identifier
}

// SetExpiry sets the expiry
func (n *Notification) SetExpiry(expiry time.Duration) {
	n.expiry = expiry
}

// Expiry returns the expiry
func (n *Notification) Expiry() time.Duration {
	return n.expiry
}

// SetPriority returns the priority
func (n *Notification) SetPriority(priority NotificationPriority) {
	n.priority = priority
}

// Priority returns the Priority
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
		return nil, fmt.Errorf("payload is larger than the %v byte limit", MaxPayloadSizeBytes)
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
