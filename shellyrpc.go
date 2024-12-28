package shellyrpc

import (
	"cmp"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"tinygo.org/x/bluetooth"
)

const SourceName = "go-shellyrpc"

type Client struct {
	// Address is the MAC address of the device to connect to.
	Address string

	// Adapter is the local bluetooth adapter to use.
	// optional, defaults to the default/first adapter
	Adapter Adapter
	// ServiceUUID is the UUID of the service to use.
	// optional
	ServiceUUID ServiceUUID
	// DataUUID is the UUID of the data characteristic, where the actual data is sent and received.
	// optional
	DataUUID DataUUID
	// RxCtrlUUID is the UUID of the RX control characteristic, where the length of the data to be received can be read.
	// optional
	RxCtrlUUID RxCtrlUUID
	// TxCtrlUUID is the UUID of the TX control characteristic, where the length of the data to be sent can be written.
	// optional
	TxCtrlUUID TxCtrlUUID

	// ConnectTimeout is the timeout for discovering and connecting to the remote device.
	// optional
	ConnectTimeout time.Duration
	// Timeout is the timeout for any operation other than connect.
	// optional
	Timeout time.Duration

	remoteDevice                     bluetooth.Device
	dataChar, txCtrlChar, rxCtrlChar bluetooth.DeviceCharacteristic
}

// Setup connects to the remote device and sets up the necessary
// characteristics to allow communication.
func (r *Client) Setup() (err error) {
	if r.Address == "" {
		return fmt.Errorf("Address is required")
	}

	mac, err := bluetooth.ParseMAC(strings.ToUpper(r.Address))
	if err != nil {
		return fmt.Errorf("parse remote address: %w", err)
	}
	remoteAddr := bluetooth.Address{MACAddress: bluetooth.MACAddress{MAC: mac}}

	adapter, err := r.Adapter.Get()
	if err != nil {
		return fmt.Errorf("get adapter: %w", err)
	}

	serviceUUID, err := r.ServiceUUID.Get()
	if err != nil {
		return fmt.Errorf("parse service UUID: %w", err)
	}

	charDataUUID, err := r.DataUUID.Get()
	if err != nil {
		return fmt.Errorf("get data characteristic UUID: %w", err)
	}
	charTxCtrlUUID, err := r.TxCtrlUUID.Get()
	if err != nil {
		return fmt.Errorf("get tx control characteristic UUID: %w", err)
	}
	charRxCtrlUUID, err := r.RxCtrlUUID.Get()
	if err != nil {
		return fmt.Errorf("get rx control characteristic UUID: %w", err)
	}

	remoteDevice, err := adapter.Connect(remoteAddr, bluetooth.ConnectionParams{
		ConnectionTimeout: bluetooth.NewDuration(r.ConnectTimeout),
		Timeout:           bluetooth.NewDuration(r.Timeout),
	})
	if err != nil {
		return fmt.Errorf("connect to device: %w", err)
	}
	defer func() {
		if err != nil {
			disconnectErr := remoteDevice.Disconnect()
			if disconnectErr != nil {
				err = fmt.Errorf("disconnect from device: %w, %w", disconnectErr, err)
			}
		}
	}()

	wantedServices := []bluetooth.UUID{serviceUUID}
	services, err := remoteDevice.DiscoverServices(wantedServices)
	if err != nil {
		return fmt.Errorf("discover services %q: %w", wantedServices, err)
	}
	service := services[0]

	wantedChars := []bluetooth.UUID{charDataUUID, charTxCtrlUUID, charRxCtrlUUID}
	chars, err := service.DiscoverCharacteristics(wantedChars)
	if err != nil {
		return fmt.Errorf("discover characteristics %q: %w", wantedChars, err)
	}

	r.remoteDevice = remoteDevice
	r.dataChar = chars[0]
	r.txCtrlChar = chars[1]
	r.rxCtrlChar = chars[2]

	return nil
}

// Teardown disconnects from the remote device.
func (r *Client) Teardown() error {
	return r.remoteDevice.Disconnect()
}

// Params are the parameters to pass to the RPC method.
type Params = map[string]any

// Result are the results returned by the RPC method.
type Result = map[string]any

type RequestFrame struct {
	ID     uint64 `json:"id"`
	Source string `json:"src"`
	Method string `json:"method"`
	Params Params `json:"params"`
}

type ResponseFrame struct {
	ID          uint64 `json:"id"`
	Destination string `json:"dst"`
	Result      Result `json:"result"`
}

// Roundtrip sends the given request frame to the device and reads the response frame.
// The high-level Call method should be preferred over this method, except when
// access to the raw frames is needed.
func (r *Client) Roundtrip(req RequestFrame) (ResponseFrame, error) {
	res := ResponseFrame{}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return res, fmt.Errorf("marshalling request: %w", err)
	}
	reqLenBytes := toBytes(uint32(len(reqBytes)))

	err = writeToChar(r.txCtrlChar, reqLenBytes)
	if err != nil {
		return res, fmt.Errorf("write request length to TX control characteristic: %w", err)
	}

	err = writeToChar(r.dataChar, reqBytes)
	if err != nil {
		return res, fmt.Errorf("write request to data characteristic: %w", err)
	}

	resLenBytes, err := readFromChar(r.rxCtrlChar, 4)
	if err != nil {
		return res, fmt.Errorf("read response length from RX control characteristic: %w", err)
	}
	resLen := int(fromBytes(resLenBytes))

	resBytes, err := readFromChar(r.dataChar, resLen)
	if err != nil {
		return res, fmt.Errorf("read response from data characteristic: %w", err)
	}

	err = json.Unmarshal(resBytes, &res)
	if err != nil {
		return res, fmt.Errorf("unmarshal response: %w", err)
	}

	return res, nil
}

// Call calls the given method with the given parameters and returns the result or an error.
// It builds the request frame, sends it to the device, and reads the response
// frame, it also checks for the correct ID and source to ensure the response
// is for this request.
func (r *Client) Call(method string, params Params) (Result, error) {
	// this is pseudo random, but should be good enough for this purpose
	id := rand.Uint64()

	req := RequestFrame{
		ID:     id,
		Source: SourceName,
		Method: method,
		Params: params,
	}

	res, err := r.Roundtrip(req)
	if err != nil {
		return nil, fmt.Errorf("roundtrip: %w", err)
	}

	if e, a := id, res.ID; e != a {
		return nil, fmt.Errorf("wrong response ID, expected: %d, got: %d", e, a)
	}
	if e, a := SourceName, res.Destination; e != a {
		return nil, fmt.Errorf("wrong response destination, expected: %s, got: %s", e, a)
	}

	return res.Result, nil
}

func readFromChar(char bluetooth.DeviceCharacteristic, length int) ([]byte, error) {
	mtu, err := char.GetMTU()
	if err != nil {
		return nil, fmt.Errorf("get MTU: %w", err)
	}

	res := []byte{}

	for length > 0 {
		buf := make([]byte, mtu)
		n, err := char.Read(buf)
		if err != nil {
			return nil, fmt.Errorf("read from characteristic: %w", err)
		}
		res = append(res, buf[:n]...)
		length -= n
	}

	return res, nil
}

func writeToChar(char bluetooth.DeviceCharacteristic, data []byte) error {
	mtu, err := char.GetMTU()
	if err != nil {
		return fmt.Errorf("get MTU: %w", err)
	}

	for len(data) > 0 {
		chunk := data
		if len(chunk) > int(mtu) {
			chunk = chunk[:mtu]
		}

		n, err := char.WriteWithoutResponse(chunk)
		if err != nil {
			return fmt.Errorf("write chunk to characteristic: %w", err)
		}
		if n != len(chunk) {
			return fmt.Errorf("write chunk to characteristic: wrote %d bytes, expected to write %d bytes", n, len(chunk))
		}

		data = data[len(chunk):]
	}

	return nil
}

// Adapter is the local bluetooth adapter name to use.
// optional, defaults to the default/first adapter ("hci0").
type Adapter string

func (a Adapter) Get() (*bluetooth.Adapter, error) {
	if a != "" {
		// https://github.com/tinygo-org/bluetooth/pull/303
		return nil, fmt.Errorf("tinygo/bluetooth does not allow other adapters")
	}
	adapter := bluetooth.DefaultAdapter
	if err := adapter.Enable(); err != nil {
		return nil, fmt.Errorf("enable adapter: %w", err)
	}
	return adapter, nil
}

const (
	// UUIDs as per documentation at
	// https://kb.shelly.cloud/knowledge-base/communicating-with-shelly-devices-via-bluetooth-lo#CommunicatingwithShellyDevicesviaBluetoothLowEnergy(BLE)UsingRPC-KeyGATTComponentsinShellyDevices
	DefaultServiceUUID = "5f6d4f53-5f52-5043-5f53-56435f49445f"

	DefaultDataUUID   = "5f6d4f53-5f52-5043-5f64-6174615f5f5f"
	DefaultTxCtrlUUID = "5f6d4f53-5f52-5043-5f74-785f63746c5f"
	DefaultRxCtrlUUID = "5f6d4f53-5f52-5043-5f72-785f63746c5f"
)

// DataUUID is the UUID of the data characteristic.
// If empty, DefaultDataUUID is used.
type DataUUID string

func (d DataUUID) Get() (bluetooth.UUID, error) {
	return asUUID(d, DefaultDataUUID)
}

// RxCtrlUUID is the UUID of the RX control characteristic.
// If empty, DefaultRxCtrlUUID is used.
type RxCtrlUUID string

func (r RxCtrlUUID) Get() (bluetooth.UUID, error) {
	return asUUID(r, DefaultRxCtrlUUID)
}

// TxCtrlUUID is the UUID of the TX control characteristic.
// If empty, DefaultTxCtrlUUID is used.
type TxCtrlUUID string

func (t TxCtrlUUID) Get() (bluetooth.UUID, error) {
	return asUUID(t, DefaultTxCtrlUUID)
}

// ServiceUUID is the UUID of the service.
// If empty, DefaultServiceUUID is used.
type ServiceUUID string

func (s ServiceUUID) Get() (bluetooth.UUID, error) {
	return asUUID(s, DefaultServiceUUID)
}

func asUUID[T ~string](uuid T, fallback string) (bluetooth.UUID, error) {
	return bluetooth.ParseUUID(cmp.Or(string(uuid), fallback))
}

func toBytes(i uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, i)
	return b
}

func fromBytes(b []byte) uint32 {
	return binary.BigEndian.Uint32(b)
}
