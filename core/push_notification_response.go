package apns

import (
	"bytes"
	"encoding/binary"
	"errors"
)

// The maximum number of seconds we're willing to wait for a response
// from the Apple Push Notification Service.
const TimeoutSeconds = 5

// APNs replies only when there is an error.
// Response len is 6 bytes:
// byte 0 : Command.
// byte 1 : Status. See `ApplePushResponseCodes`
// bytes 2-5 : Identifier. The identifier of the push notification that is the cause of the error

// Response command
// Apple response command is `AppleResponseCommand`
type PushResponseCommand uint8

const (
	AppleResponseCommand = 8
	LocalResponseCommand = 0xDD // A command used strictly locally by the library
)

// ApplePushResponseStatus is the status type (byte 1)
type ApplePushResponseStatus uint8

// Status  that APNs can reply (byte 1)
const (
	NoErrorsStatus                = 0
	ProcessingErrorStatus         = 1
	MissingDeviceTokenErrorStatus = 2
	MissingTopicErrorStatus       = 3
	MissingPayloadErrorStatus     = 4
	InvalidTokenSizeErrorStatus   = 5
	InvalidTopicSizeErrorStatus   = 6
	InvalidPayloadSizeErrorStatus = 7
	InvalidTokenErrorStatus       = 8
	ShutdownErrorStatus           = 10
	UnknownErrorStatus            = 255
)

const (
	RetryPushNotificationStatus    = 1
	CanceledPushNotificationStatus = 2
)

// This enumerates the response codes that Apple defines
// for push notification attempts.
var ApplePushResponseDescriptions = map[uint8]string{
	NoErrorsStatus:                "NO_ERRORS",
	ProcessingErrorStatus:         "PROCESSING_ERROR",
	MissingDeviceTokenErrorStatus: "MISSING_DEVICE_TOKEN",
	MissingTopicErrorStatus:       "MISSING_TOPIC",
	MissingPayloadErrorStatus:     "MISSING_PAYLOAD",
	InvalidTokenSizeErrorStatus:   "INVALID_TOKEN_SIZE",
	InvalidTopicSizeErrorStatus:   "INVALID_TOPIC_SIZE",
	InvalidPayloadSizeErrorStatus: "INVALID_PAYLOAD_SIZE",
	InvalidTokenErrorStatus:       "INVALID_TOKEN",
	ShutdownErrorStatus:           "SHUTDOWN",
	UnknownErrorStatus:            "UNKNOWN",
}

// PushNotificationResponse details what Apple had to say, if anything.
type PushNotificationResponse struct {
	Identifier      uint32
	Success         bool
	ResponseCommand PushResponseCommand
	ResponseStatus  ApplePushResponseStatus
	AppleResponse   string // Legacy field
	Error           error  // Legacy field
}

// NewPushNotificationResponse creates and returns a new PushNotificationResponse
// structure; it defaults to being unsuccessful at first.
func NewPushNotificationResponse(pn *PushNotification) *PushNotificationResponse {
	return &PushNotificationResponse{Identifier: pn.Identifier, Success: false}
}

func (pnr *PushNotificationResponse) FromRawAppleResponse(r []byte) {

	pnr.AppleResponse = ApplePushResponseDescriptions[r[1]]

	if r[1] == NoErrorsStatus { // No error, so timeout
		pnr.Success = true
		pnr.Error = nil
	} else {
		pnr.Success = false
		pnr.Error = errors.New(pnr.AppleResponse)

		pnr.ResponseCommand = PushResponseCommand(r[0])
		pnr.ResponseStatus = ApplePushResponseStatus(r[1])
		binary.Read(bytes.NewBuffer(r[2:]), binary.BigEndian, &(pnr.Identifier))
	}

}
