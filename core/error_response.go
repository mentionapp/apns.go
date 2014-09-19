package apns

import (
	"encoding/binary"
	"fmt"
)

// ErrorResponseCommand represents the Command field of error-response packets
type ErrorResponseCommand uint8

// ErrorResponseStatus represents to Status field of error-response packets
type ErrorResponseStatus uint8

// ErrorResponse represents an APNS error-response packet
type ErrorResponse struct {
	Command    ErrorResponseCommand
	Status     ErrorResponseStatus
	Identifier uint32
}

// Known values of ErrorResponseCommand
const (
	ErrorCommand ErrorResponseCommand = 8
)

// Knwon values of ErrorResponseStatus
const (
	NoErrorsStatus                ErrorResponseStatus = 0
	ProcessingErrorStatus         ErrorResponseStatus = 1
	MissingDeviceTokenErrorStatus ErrorResponseStatus = 2
	MissingTopicErrorStatus       ErrorResponseStatus = 3
	MissingPayloadErrorStatus     ErrorResponseStatus = 4
	InvalidTokenSizeErrorStatus   ErrorResponseStatus = 5
	InvalidTopicSizeErrorStatus   ErrorResponseStatus = 6
	InvalidPayloadSizeErrorStatus ErrorResponseStatus = 7
	InvalidTokenErrorStatus       ErrorResponseStatus = 8
	ShutdownErrorStatus           ErrorResponseStatus = 10
	UnknownErrorStatus            ErrorResponseStatus = 255
)

const ErrorResponseLength = 6

var errorResponseStatusNames = map[ErrorResponseStatus]string{
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

func (e ErrorResponseStatus) String() string {
	if s, ok := errorResponseStatusNames[e]; ok {
		return s
	}
	return "INVALID"
}

func DecodeErrorResponse(r []byte) (*ErrorResponse, error) {

	if len(r) != ErrorResponseLength {
		return nil, fmt.Errorf("Invalid buffer length: expected %v bytes, got %v", ErrorResponseLength, len(r))
	}

	er := &ErrorResponse{
		Command:    ErrorResponseCommand(r[0]),
		Status:     ErrorResponseStatus(r[1]),
		Identifier: binary.BigEndian.Uint32(r[2:]),
	}

	return er, nil
}
