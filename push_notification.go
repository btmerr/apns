package apns

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/rand"
	"strconv"
	"time"
)

// Push commands always start with command value 2.
const PUSH_COMMAND_VALUE = 2

// Your total notification payload cannot exceed 256 bytes.
const MAX_PAYLOAD_SIZE_BYTES = 256

// Every push notification gets a pseudo-unique identifier;
// this establishes the upper boundary for it. Apple will return
// this identifier if there is an issue sending your notification.
const IDENTIFIER_UBOUND = 9999

const (
	DeviceTokenItemid = 1
	PayloadItemid = 2
	NotificationIdentifierItemid = 3
	ExpirationDateItemid = 4
	PriorityItemid = 5
)

const (
	DeviceTokenLength = 32
	NotificationIdentifierLength = 4
	ExpirationDateLength = 4
	PriorityLength = 1
)

// Alert is an interface here because it supports either a string
// or a dictionary, represented within by an AlertDictionary struct.
type Payload struct {
	Alert interface{} `json:"alert,omitempty"`
	Badge int         `json:"badge,omitempty"`
	Sound string      `json:"sound,omitempty"`
}

// Constructor.
func NewPayload() *Payload {
	return new(Payload)
}

// From the APN docs: "Use the ... alert dictionary in general only if you absolutely need to."
// The AlertDictionary is suitable for specific localization needs.
type AlertDictionary struct {
	Body         string   `json:"body,omitempty"`
	ActionLocKey string   `json:"action-loc-key,omitempty"`
	LocKey       string   `json:"loc-key,omitempty"`
	LocArgs      []string `json:"loc-args,omitempty"`
	LaunchImage  string   `json:"launch-image,omitempty"`
}

// Constructor.
func NewAlertDictionary() *AlertDictionary {
	return new(AlertDictionary)
}

// The PushNotification is the wrapper for the Payload.
// The length fields are computed in ToBytes() and aren't represented here.
type PushNotification struct {
	Identifier  int32
	Expiry      uint32
	DeviceToken string
	payload     map[string]interface{}
	Priority    uint8
}

// Constructor. Also initializes the pseudo-random identifier.
func NewPushNotification() (pn *PushNotification) {
	pn = new(PushNotification)
	pn.payload = make(map[string]interface{})
	pn.Identifier = rand.New(rand.NewSource(time.Now().UnixNano())).Int31n(IDENTIFIER_UBOUND)
	pn.Priority = 10
	return
}

func (this *PushNotification) AddPayload(p *Payload) {
	// This deserves some explanation.
	//
	// Setting an exported field of type int to 0
	// triggers the omitempty behavior if you've set it.
	// Since the badge is optional, we should omit it if
	// it's not set. However, we want to include it if the
	// value is 0, so there's a hack in push_notification.go
	// that exploits the fact that Apple treats -1 for a
	// badge value as though it were 0 (i.e. it clears the
	// badge but doesn't stop the notification from going
	// through successfully.)
	//
	// Still a hack though :)
	if p.Badge == 0 {
		p.Badge = -1
	}
	this.Set("aps", p)
}

func (this *PushNotification) Get(key string) interface{} {
	return this.payload[key]
}

func (this *PushNotification) Set(key string, value interface{}) {
	this.payload[key] = value
}

func (this *PushNotification) PayloadJSON() ([]byte, error) {
	return json.Marshal(this.payload)
}

func (this *PushNotification) PayloadString() (string, error) {
	j, err := this.PayloadJSON()
	return string(j), err
}

// Returns a byte array of the complete PushNotification struct. This array
// is what should be transmitted to the APN Service.
func (this *PushNotification) ToBytes() ([]byte, error) {
	token, err := hex.DecodeString(this.DeviceToken)
	if err != nil {
		return nil, err
	}
	payload, err := this.PayloadJSON()
	if err != nil {
		return nil, err
	}
	if len(payload) > MAX_PAYLOAD_SIZE_BYTES {
		return nil, errors.New("payload is larger than the " + strconv.Itoa(MAX_PAYLOAD_SIZE_BYTES) + " byte limit")
	}

	frameBuffer := new(bytes.Buffer)
	binary.Write(frameBuffer, binary.BigEndian, uint8(DeviceTokenItemid))
	binary.Write(frameBuffer, binary.BigEndian, uint16(DeviceTokenLength))
	binary.Write(frameBuffer, binary.BigEndian, token)
	binary.Write(frameBuffer, binary.BigEndian, uint8(PayloadItemid))
	binary.Write(frameBuffer, binary.BigEndian, uint16(len(payload)))
	binary.Write(frameBuffer, binary.BigEndian, payload)
	binary.Write(frameBuffer, binary.BigEndian, uint8(NotificationIdentifierItemid))
	binary.Write(frameBuffer, binary.BigEndian, uint16(NotificationIdentifierLength))
	binary.Write(frameBuffer, binary.BigEndian, this.Identifier)
	binary.Write(frameBuffer, binary.BigEndian, uint8(ExpirationDateItemid))
	binary.Write(frameBuffer, binary.BigEndian, uint16(ExpirationDateLength))
	binary.Write(frameBuffer, binary.BigEndian, this.Expiry)
	binary.Write(frameBuffer, binary.BigEndian, uint8(PriorityItemid))
	binary.Write(frameBuffer, binary.BigEndian, uint16(PriorityLength))
	binary.Write(frameBuffer, binary.BigEndian, this.Priority)

	buffer := bytes.NewBuffer([]byte{})
	binary.Write(buffer, binary.BigEndian, uint8(PUSH_COMMAND_VALUE))
	binary.Write(buffer, binary.BigEndian, uint32(frameBuffer.Len()))
	binary.Write(buffer, binary.BigEndian, frameBuffer.Bytes())
	return buffer.Bytes(), nil
}
