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

// PersistentClient opens a persistent connexion with the gateway
type PersistentClient struct {
	client *Client
	conn   net.Conn
	ip     string
}

// NewPersistentClient create a new persistent connection to
func NewPersistentClient(gateway, ip, certificateFile, keyFile string) (*PersistentClient, error) {

	var c *PersistentClient = &PersistentClient{}
	c.client = NewClient(gateway, certificateFile, keyFile)
	c.ip = ip
	err := c.Connect()
	if err != nil {
		return nil, err
	}
	return c, err
}

//
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

// reconnect forces a new connection to the gateway
// If a connection exists it is closed before the creation of a new one
func (c *PersistentClient) Reconnect() error {

	var cert tls.Certificate
	var err error

	if c.conn != nil {
		c.closeAndNil()
	}

	if len(c.client.CertificateBase64) == 0 && len(c.client.KeyBase64) == 0 {
		// The user did not specify raw block contents, so check the filesystem.
		cert, err = tls.LoadX509KeyPair(c.client.CertificateFile, c.client.KeyFile)
	} else {
		// The user provided the raw block contents, so use that.
		cert, err = tls.X509KeyPair([]byte(c.client.CertificateBase64), []byte(c.client.KeyBase64))
	}
	if err != nil {
		return err
	}
	gatewayParts := strings.Split(c.client.Gateway, ":")
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
	log.Printf("Address of %s is %s", c.client.Gateway, c.conn.RemoteAddr().String())
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
		_, err := c.conn.Read(buffer)
		if err != nil {
			log.Println("Read error is ", err)
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

func (c *PersistentClient) Close() {
	c.closeAndNil()
}

// Close and nil a conn
// Used to not forget to nil the connection
func (c *PersistentClient) closeAndNil() {
	log.Printf("Closing %s at address %s", c.client.Gateway, c.conn.RemoteAddr().String())
	c.conn.Close()
	c.conn = nil
}
