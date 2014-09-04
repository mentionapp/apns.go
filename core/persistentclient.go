package apns

import (
	"crypto/tls"
	"io"
	"log"
	"net"
	"strings"
	"time"

	"code.google.com/p/go.net/context"
)

type client struct {
	gateway           string
	certificateFile   string
	certificateBase64 string
	keyFile           string
	keyBase64         string
}

// PersistentClient opens a persistent connexion with the gateway
type PersistentClient struct {
	client *client
	conn   net.Conn
	ip     string
}

// NewPersistentClient creates a new persistent connection to the APNs servers
func NewPersistentClient(gateway, ip, certificateFile, keyFile string) (*PersistentClient, error) {

	var c *PersistentClient = &PersistentClient{}
	c.client = &client{gateway: gateway, certificateFile: certificateFile, keyFile: keyFile}
	c.ip = ip
	err := c.Connect()
	if err != nil {
		return nil, err
	}
	return c, err
}

// Connect connects the persistent client to one of the APNs server
// If the connection is already established and was not closed, it does nothing.
func (c *PersistentClient) Connect() error {

	// Check if there is already a connection
	if c.conn != nil {
		// If connection is not nil it should be ok
		// c.conn is set to nil when there is an error on read or write
		// because the gateway close it anyway in this case
		return nil
	}
	return c.Reconnect()
}

// Reconnect forces a new connection to the gateway
// If a connection exists it is closed before the creation of a new one
func (c *PersistentClient) Reconnect() error {

	var cert tls.Certificate
	var err error

	if c.conn != nil {
		c.closeAndNil()
	}

	if len(c.client.certificateBase64) == 0 && len(c.client.keyBase64) == 0 {
		// The user did not specify raw block contents, so check the filesystem.
		cert, err = tls.LoadX509KeyPair(c.client.certificateFile, c.client.keyFile)
	} else {
		// The user provided the raw block contents, so use that.
		cert, err = tls.X509KeyPair([]byte(c.client.certificateBase64), []byte(c.client.keyBase64))
	}
	if err != nil {
		return err
	}
	gatewayParts := strings.Split(c.client.gateway, ":")
	conf := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ServerName:   gatewayParts[0],
	}
	if c.ip == "" { // If the ip is not provided pick one
		ip, err := pickGatewayIP(gatewayParts[0])
		if err != nil {
			return err
		}
		c.ip = ip
	}
	conn, err := net.Dial("tcp", c.ip+":"+gatewayParts[1])
	if err != nil {
		return err
	}
	tlsConn := tls.Client(conn, conf)
	err = tlsConn.Handshake()
	if err != nil {
		conn.Close()
		return err
	}
	c.conn = net.Conn(tlsConn)
	log.Printf("Address of %s is %s", c.client.gateway, c.conn.RemoteAddr().String())
	return nil
}

// Send sends push notification to the APNs.
func (c *PersistentClient) Send(ctx context.Context, pn *PushNotification) *PushNotificationResponse {

	resp := NewPushNotificationResponse(pn)
	payload, err := pn.ToBytes()
	if err != nil {
		resp.Success = false
		resp.Error = err
		return resp
	}

	_, err = c.conn.Write(payload)
	if err != nil {
		resp.Success = false
		resp.ResponseCommand = LocalResponseCommand
		resp.ResponseStatus = RetryPushNotificationStatus
		resp.Error = err
		return resp
	}
	log.Println("Sending push notification with ID", pn.Identifier)

	// This channel will contain the raw response
	// from Apple in the event of a failure.
	responseChannel := make(chan []byte, 1)
	go func() {
		c.conn.SetReadDeadline(time.Now().Add(time.Second * TimeoutSeconds))
		buffer := make([]byte, 6)
		n, err := c.conn.Read(buffer)
		if n != 6 && err != nil {
			buffer[0] = LocalResponseCommand
			e, ok := err.(net.Error)
			switch {
			case err == io.EOF: // Socket has been closed
				buffer[1] = RetryPushNotificationStatus
			case ok && e.Timeout(): // There is an error and it is a timeout
				buffer[1] = NoErrorsStatus
			default:
				buffer[1] = UnknownErrorStatus
			}
		}
		responseChannel <- buffer
	}()

	select {
	case <-ctx.Done():
		<-responseChannel // Wait for the read to end.
		resp.Success = false
		resp.ResponseCommand = LocalResponseCommand
		resp.ResponseStatus = CanceledPushNotificationStatus
		resp.Error = ctx.Err()
	case r := <-responseChannel:
		resp.FromRawAppleResponse(r)
	}
	return resp
}

// Close closes the persistent client
func (c *PersistentClient) Close() {
	c.closeAndNil()
}

// closeAndNil closes a persistent connection and set the conn to nil
func (c *PersistentClient) closeAndNil() {
	// Used to not forget to nil the connection
	log.Printf("Closing %s at address %s", c.client.gateway, c.conn.RemoteAddr().String())
	c.conn.Close()
	c.conn = nil
}
