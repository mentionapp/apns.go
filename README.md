# apns

Utilities for Apple Push Notification and Feedback Services.

[![GoDoc](https://godoc.org/github.com/gsempe/apns?status.png)](https://godoc.org/github.com/gsempe/apns)

## Installation

`go get github.com/gsempe/apns/core`

## Documentation

- [APNS package documentation](http://godoc.org/github.com/gsempe/apns)
- [Information on the APN JSON payloads](http://developer.apple.com/library/mac/#documentation/NetworkingInternet/Conceptual/RemoteNotificationsPG/Chapters/ApplePushService.html)
- [Information on the APN binary protocols](http://developer.apple.com/library/ios/#documentation/NetworkingInternet/Conceptual/RemoteNotificationsPG/Chapters/CommunicatingWIthAPS.html)
- [Information on APN troubleshooting](http://developer.apple.com/library/ios/#technotes/tn2265/_index.html)

## Usage

### Creating push notification and payload
```go
package main

import (
  "fmt"
  apns "github.com/gsempe/apns"
)

func main() {
  payload := apns.NewPayload()
  payload.Alert = "Hello, world!"
  payload.Badge = 42
  payload.Sound = "bingbong.aiff"

  pn := apns.NewPushNotification()
  pn.AddPayload(payload)

  alert, _ := pn.PayloadString()
  fmt.Println(alert)
}
```

`Println` output:

```json
{
  "aps": {
    "alert": "Hello, world!",
    "badge": 42,
    "sound": "bingbong.aiff"
  }
}
```

### Sending a notification

todo(gse): Add documentation to send push notifications

## Advanced push notifications formats

### Using an alert dictionary for complex payloads
```go
package main

import (
  "fmt"
  apns "github.com/anachronistic/apns"
)

func main() {
  args := make([]string, 1)
  args[0] = "localized args"

  dict := apns.NewAlertDictionary()
  dict.Body = "Alice wants Bob to join in the fun!"
  dict.ActionLocKey = "Play a Game!"
  dict.LocKey = "localized key"
  dict.LocArgs = args
  dict.LaunchImage = "image.jpg"

  payload := apns.NewPayload()
  payload.Alert = dict
  payload.Badge = 42
  payload.Sound = "bingbong.aiff"

  pn := apns.NewPushNotification()
  pn.AddPayload(payload)

  alert, _ := pn.PayloadString()
  fmt.Println(alert)
}
```

#### Returns
```json
{
  "aps": {
    "alert": {
      "body": "Alice wants Bob to join in the fun!",
      "action-loc-key": "Play a Game!",
      "loc-key": "localized key",
      "loc-args": [
        "localized args"
      ],
      "launch-image": "image.jpg"
    },
    "badge": 42,
    "sound": "bingbong.aiff"
  }
}
```

### Setting custom properties
```go
package main

import (
  "fmt"
  apns "github.com/anachronistic/apns"
)

func main() {
  payload := apns.NewPayload()
  payload.Alert = "Hello, world!"
  payload.Badge = 42
  payload.Sound = "bingbong.aiff"

  pn := apns.NewPushNotification()
  pn.AddPayload(payload)

  pn.Set("foo", "bar")
  pn.Set("doctor", "who?")
  pn.Set("the_ultimate_answer", 42)

  alert, _ := pn.PayloadString()
  fmt.Println(alert)
}
```

#### Returns
```json
{
  "aps": {
    "alert": "Hello, world!",
    "badge": 42,
    "sound": "bingbong.aiff"
  },
  "doctor": "who?",
  "foo": "bar",
  "the_ultimate_answer": 42
}
```

