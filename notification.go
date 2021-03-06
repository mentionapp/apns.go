package apns

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// MaxPayloadLen is the maximum allowed payload length (after JSON encoding)
const MaxPayloadLen = 256

// Interface for universal notification payload
type Topic interface {
	Bytes() ([]byte, error)
}

// Payload represents a notification payload
type Payload map[string]interface{}

// NotificationIdentifier represents a notification identifier
type NotificationIdentifier uint32

// NotificationPriority represents a notification priority
type NotificationPriority uint8

// Notification represents a notification
type Notification struct {
	deviceToken string
	payload     Topic
	identifier  *NotificationIdentifier
	expiry      time.Time
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
	// ImmediatePriority sets the push message to be sent immediately. This is
	// the default.
	ImmediatePriority NotificationPriority = 10
	// PowerSavingPriority sets the push message to be sent at a time that conserves power on the device receiving it
	PowerSavingPriority NotificationPriority = 5
)

const pushCommandValue uint8 = 2

// Constants related to the payload fields and their lengths.
const (
	deviceTokenItemid            uint8  = 1
	payloadItemid                uint8  = 2
	notificationIdentifierItemid uint8  = 3
	expirationDateItemid         uint8  = 4
	priorityItemid               uint8  = 5
	deviceTokenLength            uint16 = 32
	notificationIdentifierLength uint16 = 4
	expirationDateLength         uint16 = 4
	priorityLength               uint16 = 1
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
// MaxPayloadLen bytes
// deprecated
func (p Payload) ToJSON() ([]byte, error) {
	return p.Bytes()
}

// ToJSON encodes the Payload to JSON. The encoded payload cannot exceed
// MaxPayloadLen bytes
func (p Payload) Bytes() ([]byte, error) {
	return json.Marshal(p)
}

func (p Payload) aps() map[string]interface{} {
	if e, ok := p["aps"]; ok {
		if aps, ok := e.(map[string]interface{}); ok {
			return aps
		}
	}
	aps := map[string]interface{}{}
	p["aps"] = aps
	return aps
}

// NewNotification creates a new Notification
func NewNotification() *Notification {
	n := &Notification{}
	n.payload = Payload{}
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

// SetPayload sets the Payload.
func (n *Notification) SetPayload(payload Topic) {
	n.payload = payload
}

// Payload returns the Payload
func (n *Notification) Payload() Topic {
	return n.payload
}

// SetIdentifier sets a custom identifier. Two notifications sent to the
// same Sender must have different identifiers. Sender automatically chooses an
// identifier if one was not set.
func (n *Notification) SetIdentifier(identifier NotificationIdentifier) {
	n.identifier = &identifier
}

// Identifier returns the Identifier
func (n *Notification) Identifier() NotificationIdentifier {
	if n.identifier != nil {
		return *n.identifier
	}
	return 0
}

// HasIdentifier returns whether the Identifier has been set
func (n *Notification) HasIdentifier() bool {
	return n.identifier != nil
}

// SetExpiry sets the expiry. Fractions of seconds are truncated. APNS discards
// the notification if it wasn't able to send it after this duration. An expiry
// of 0 means that the notification is discarded immediately by APNS if it can
// not be sent (the is the default).
func (n *Notification) SetExpiry(expiry time.Time) {
	n.expiry = expiry
}

// Expiry returns the expiry
func (n *Notification) Expiry() time.Time {
	return n.expiry
}

// SetPriority sets the priority. The default is ImmediatePriority.
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

	payload, err := n.payload.Bytes()
	if err != nil {
		return nil, err
	}

	if len(payload) > MaxPayloadLen {
		return nil, fmt.Errorf("payload is larger than the %v byte limit", MaxPayloadLen)
	}

	BE := binary.BigEndian

	frameBuffer := &bytes.Buffer{}

	binary.Write(frameBuffer, BE, deviceTokenItemid)
	binary.Write(frameBuffer, BE, deviceTokenLength)
	binary.Write(frameBuffer, BE, token)

	binary.Write(frameBuffer, BE, payloadItemid)
	binary.Write(frameBuffer, BE, uint16(len(payload)))
	binary.Write(frameBuffer, BE, payload)

	if n.identifier == nil {
		return nil, fmt.Errorf("identifier was not set")
	}
	binary.Write(frameBuffer, BE, notificationIdentifierItemid)
	binary.Write(frameBuffer, BE, notificationIdentifierLength)
	binary.Write(frameBuffer, BE, *n.identifier)

	binary.Write(frameBuffer, BE, expirationDateItemid)
	binary.Write(frameBuffer, BE, expirationDateLength)
	binary.Write(frameBuffer, BE, uint32(n.expiry.Unix()))

	binary.Write(frameBuffer, BE, priorityItemid)
	binary.Write(frameBuffer, BE, priorityLength)
	binary.Write(frameBuffer, BE, n.priority)

	buffer := bytes.NewBuffer([]byte{})
	binary.Write(buffer, BE, pushCommandValue)
	binary.Write(buffer, BE, uint32(frameBuffer.Len()))
	binary.Write(buffer, BE, frameBuffer.Bytes())

	return buffer.Bytes(), nil
}
