# Shelly RPC over BLE & golang

## What

Allows you to directly, without their gateway, communicate with Shelly BLU
devices, e.g. a [Shelly BLU TRV][trv], via their bluetooth RPC mechanism. It is
implemented according the documentation over
[here](https://kb.shelly.cloud/knowledge-base/communicating-with-shelly-devices-via-bluetooth-lo).

While it is tailored towards my use with Shelly devices, it might be useful to
communicate with other Mongoose OS based devices, too. Some notes about RPC for
Mongoose OS can be found
[here](https://mongoose-os.com/docs/mongoose-os/api/rpc/rpc-gatts.md).

Under the hood [TinyGo]'s [Bluetooth API][bt] is used.

Note: You have to pair the device/sensor before you can use this module, you
can use e.g. `bluetoothctl` for that.

[trv]: https://www.shelly.com/products/shelly-blu-trv-single-pack
[TinyGo]: https://tinygo.org/
[bt]: https://github.com/tinygo-org/bluetooth

## Status

Experimental. Seems to work for my Shelly BLU TRVs.

## Example

There is an example application at
[./cmd/shellyrpc/main.go](cmd/shellyrpc/main.go) which calls an RPC method
(with optional parameters) and dumps the response on standard out:

```terminal
: go run ./cmd/shellyrpc/main.go --help
Usage of /tmp/go-build4271337887/b001/exe/main:
  -addr string
        Shelly device address (default "f8:44:77:21:12:55")
  -method string
        RPC method to call (default "Shelly.GetConfig")
  -params string
        RPC method parameters as JSON blob (default "null")
: go run ./cmd/shellyrpc/main.go
{
  "sys": {
    "ble": {
      "beacon_count": 5,
      "interval_ms": 333
    },
    "cfg_rev": 6,
    "device": {
      "name": ""
    },
    "location": {
      "lat": 0,
      "lon": 0
    },
    "ui": {
      "brightness": 7,
      "flip": false,
      "lock": false,
      "t_units": "C"
    }
  },
  "temperature:0": {
    "id": 0,
    "offset_C": 0
  },
  "trv:0": {
    "default_boost_duration": 1800,
    "default_override_duration": 2147483647,
    "default_override_target_C": 8,
    "enable": true,
    "flags": [
      "auto_calibrate",
      "anticlog"
    ],
    "id": 0,
    "min_valve_position": 0,
    "override_enable": true
  }
}
: 
```
