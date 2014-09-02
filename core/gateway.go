package apns

import (
	"errors"
	"log"
	"math"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	"code.google.com/p/go.net/context"
)

const (
	gatewayName = "gateway.push.apple.com"
	gatewayPort = "2195"

	gatewaySandboxName = "gateway.sandbox.push.apple.com"
	gatewaySandboxPort = "2195"
)

type gatewayIPs []string

var (
	mu               sync.Mutex // Protect acces to `gatewayIP` or to `gatewaySandboxIP`
	gatewayIP        gatewayIPs // Cache of the IPs of the production APNs
	gatewaySandboxIP gatewayIPs // Cache of the IPs of the sandbox APNs
)

// ips is useed to balance senders connection between all the gateway IPs
type ips struct {
	mu    sync.Mutex      // Protect acces to `ipMap`
	ipMap map[string]int8 // Save the usage of each IP available
}

// Gateway keeps all the informations needed to communicate with the APNs
type Gateway struct {
	gateway   string                         // The gateway name could be either `gatewayName`, `gatewaySandboxName` or a custom host:port
	gips      ips                            // save the balance of used IPs for the `gateway`
	responses chan *PushNotificationResponse // Channel of errors, directly from senders
	onError   OnErrorCallback                // Error callback to execute client code on error
	senders   []*Sender                      // Array of potential senders
}

// OnErrorCallback functions are called to let the library client react when an error occured
type OnErrorCallback func(*PushNotificationResponse)

func init() {
	// Initialize at startup the rand seed
	rand.Seed(time.Now().UTC().UnixNano())
}

// NewGateway creates a new gateway interface to the Apple APNs production servers
func NewGateway(ctx context.Context, certificateFile, keyFile string) (*Gateway, error) {

	gw := gatewayName + ":" + gatewayPort
	return NewCustomGateway(ctx, gw, certificateFile, keyFile)
}

// NewSandboxGateway creates a new gateway interface to the Apple APNs sandbox servers
func NewSandboxGateway(ctx context.Context, certificateFile, keyFile string) (*Gateway, error) {

	gw := gatewaySandboxName + ":" + gatewaySandboxPort
	return NewCustomGateway(ctx, gw, certificateFile, keyFile)
}

// NewCustomGateway create a client interface to a custom APNs server
// `gateway` format must be "hostname:port"
func NewCustomGateway(ctx context.Context, gateway, certificateFile, keyFile string) (*Gateway, error) {

	g := &Gateway{gateway: gateway}
	return g.newGateway(ctx, certificateFile, keyFile)
}

// Send uses one of the gateway sender to send the push notification
func (g *Gateway) Send(pn *PushNotification) {

	min := 0
	max := len(g.senders)
	n := rand.Intn(max-min) + min
	g.senders[n].Send(pn)
}

// Errors gives feedback to the library client on which push notifications got errors
// The library client has to provide a callback via this method to get error informations.
func (g *Gateway) Errors(onError OnErrorCallback) {

	g.onError = onError
}

// newGateway does all the gateway initialisation once the gateway name is known
func (g *Gateway) newGateway(ctx context.Context, certificateFile, keyFile string) (*Gateway, error) {

	gips, err := lookupGateway(g.gateway)
	if nil != err {
		return nil, err
	}
	ipMap := map[string]int8{}
	for _, ip := range gips {
		ipMap[ip] = 0
	}
	g.gips = ips{ipMap: ipMap}
	g.responses = make(chan *PushNotificationResponse)
	g.senders = []*Sender{}
	// TODO GSE: Enable the possibilty to choose the number of senders
	err = g.newSender(ctx, certificateFile, keyFile)
	if err != nil {
		return nil, err
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case pnr := <-g.responses:
				if g.onError != nil {
					go g.onError(pnr)
				}
			}
		}
	}()
	return g, nil
}

// newSender creates a sender and attach it to the gateway
func (g *Gateway) newSender(ctx context.Context, certificateFile, keyFile string) error {

	ip := g.balanceGatewayIps()
	s, err := NewSender(ctx, g.gateway, ip, certificateFile, keyFile)
	if err != nil {
		return err
	}
	s.pnrec = g.responses
	g.senders = append(g.senders, s)
	return nil
}

// balanceGatewayIps chooses the best ip for the gateway
func (g *Gateway) balanceGatewayIps() string {

	// TODO(GSE): Should probably get feedbacks from the clients to minus one an IP when it is freed by a sender
	minNbUsage := int8(math.MaxInt8) // Initialize it at the max value possible to track the best choice available
	bestip := ""
	g.gips.mu.Lock()
	defer g.gips.mu.Unlock()
	for ip, usage := range g.gips.ipMap {
		if usage < minNbUsage {
			minNbUsage = usage
			bestip = ip
		}
	}
	if bestip != "" {
		g.gips.ipMap[bestip] = g.gips.ipMap[bestip] + 1
	}
	return bestip
}

// lookupGateway gets the gateway IPs from the gateway name without the colon and the port
// Useful to load balance over the different APNS servers behind the gateway name
func lookupGateway(gateway string) (gatewayIPs, error) {

	gatewayParts := strings.Split(gateway, ":")
	if len(gatewayParts) == 2 {
		gateway = gatewayParts[0]
	}
	ips, err := net.LookupIP(gateway)
	if err != nil {
		return nil, err
	}
	mu.Lock()
	defer mu.Unlock()
	if gateway == gatewayName {
		gatewayIP = make(gatewayIPs, len(ips))
		for i := 0; i < len(ips); i++ {
			ip := ips[i]
			gatewayIP[i] = ip.String()
		}
		return gatewayIP, nil
	}
	if gateway == gatewaySandboxName {
		gatewaySandboxIP = make(gatewayIPs, len(ips))
		for i := 0; i < len(ips); i++ {
			ip := ips[i]
			gatewaySandboxIP[i] = ip.String()
		}
		return gatewaySandboxIP, nil
	}
	return nil, errors.New("lookupGateway reach an invalid state")
}

// cachedLookupGateway gets the cached IPs from the gateway name without the colon and the port
// If there is no cache retrieves the IPs
func cachedLookupGateway(gateway string) (gatewayIPs, error) {

	if gateway == gatewayName && len(gatewayIP) > 0 {
		return gatewayIP, nil
	}
	if gateway == gatewaySandboxName && len(gatewaySandboxIP) > 0 {
		return gatewaySandboxIP, nil
	}
	return lookupGateway(gateway)
}

// pickGatewayIP picks an IP of the gateway
func pickGatewayIP(gateway string) (string, error) {

	ips, err := cachedLookupGateway(gateway)
	if err != nil {
		return "", err
	}
	// TODO(GSE): Make the pick better
	// IPs picks should be balanced between all the IPs available for the gateway
	//
	min := 0
	max := len(ips)
	n := rand.Intn(max-min) + min
	return ips[n], nil
}
