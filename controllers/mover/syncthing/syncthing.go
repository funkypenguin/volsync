/*
Copyright 2022 The VolSync authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/
package syncthing

import (
	"crypto/rand"
	"fmt"
	"regexp"

	"github.com/backube/volsync/api/v1alpha1"
	"github.com/backube/volsync/controllers/mover/syncthing/api"
	"github.com/syncthing/syncthing/lib/config"
	"github.com/syncthing/syncthing/lib/protocol"
)

// updateSyncthingDevices Updates the Syncthing's connected devices with the provided peerList.
// An error may be encountered when reading the DeviceID from a string.
func updateSyncthingDevices(peerList []v1alpha1.SyncthingPeer,
	syncthing *api.Syncthing) error {
	if syncthing == nil {
		return fmt.Errorf("syncthing cannot be nil")
	}
	newDevices := []config.DeviceConfiguration{}
	// add myself and introduced devices to the device list
	for _, device := range syncthing.Configuration.Devices {
		if device.DeviceID.GoString() == syncthing.MyID() || device.IntroducedBy.GoString() != "" {
			newDevices = append(newDevices, device)
		}
	}
	// Add the devices from the peerList to the device list
	for _, device := range peerList {
		deviceID, err := protocol.DeviceIDFromString(device.ID)
		if err != nil {
			return err
		}
		stDeviceToAdd := config.DeviceConfiguration{
			DeviceID:   deviceID,
			Addresses:  []string{device.Address},
			Introducer: device.Introducer,
		}
		newDevices = append(newDevices, stDeviceToAdd)
	}
	syncthing.Configuration.Devices = newDevices
	syncthing.ShareFoldersWithDevices(newDevices)
	return nil
}

// syncthingNeedsReconfigure Determines whether the given nodeList differs from Syncthing's internal devices,
// and returns 'true' if the Syncthing API must be reconfigured, 'false' otherwise.
func syncthingNeedsReconfigure(
	nodeList []v1alpha1.SyncthingPeer,
	syncthing *api.Syncthing,
) bool {
	// check if the syncthing nodelist diverges from the current syncthing devices
	var newDevices map[string]v1alpha1.SyncthingPeer = map[string]v1alpha1.SyncthingPeer{
		// initialize the map with the self node
		syncthing.MyID(): {
			ID:      syncthing.MyID(),
			Address: "",
		},
	}

	// add all of the other devices in the provided nodeList
	for _, device := range nodeList {
		// avoid self
		if device.ID == syncthing.MyID() {
			continue
		}
		newDevices[device.ID] = device
	}

	// create a map for current devices
	var currentDevs map[string]v1alpha1.SyncthingPeer = map[string]v1alpha1.SyncthingPeer{
		// initialize the map with the self node
		syncthing.MyID(): {
			ID:      syncthing.MyID(),
			Address: "",
		},
	}
	// add the rest of devices to the map
	for _, device := range syncthing.Configuration.Devices {
		// ignore self and introduced devices
		if device.DeviceID.GoString() == syncthing.MyID() || device.IntroducedBy.GoString() != "" {
			continue
		}

		currentDevs[device.DeviceID.GoString()] = v1alpha1.SyncthingPeer{
			ID:      device.DeviceID.GoString(),
			Address: device.Addresses[0],
		}
	}

	// check if the syncthing nodelist diverges from the current syncthing devices
	for _, device := range newDevices {
		if _, ok := currentDevs[device.ID]; !ok {
			return true
		}
	}
	for _, device := range currentDevs {
		if _, ok := newDevices[device.ID]; !ok {
			return true
		}
	}
	return false
}

// GenerateRandomBytes Generates random bytes of the given length using the OS's RNG.
func GenerateRandomBytes(length int) ([]byte, error) {
	// generates random bytes of given length
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// GenerateRandomString Generates a random string of ASCII characters excluding control characters
// 0-31, 32 (space), and 127.
// the given length using the OS's RNG.
func GenerateRandomString(length int) (string, error) {
	// generate a random string
	b, err := GenerateRandomBytes(length)
	if err != nil {
		return "", err
	}

	// construct string by mapping the randomly generated bytes into
	// a range of acceptable characters
	var lowerBound byte = 33
	var upperBound byte = 126
	var acceptableRange = upperBound - lowerBound + 1

	// generate the string by mapping [0, 255] -> [33, 126]
	var acceptableBytes = []byte{}
	for i := 0; i < len(b); i++ {
		// normalize number to be in the range [33, 126] inclusive
		acceptableByte := (b[i] % acceptableRange) + lowerBound
		acceptableBytes = append(acceptableBytes, acceptableByte)
	}
	return string(acceptableBytes), nil
}

// asTCPAddress Accepts an address of some form and returns it with a TCP prefix if none exist yet.//
// If the address already contains a prefix, then it is simply returned.
//
// See: https://forum.syncthing.net/t/specifying-protocols-without-global-announce-or-relay/18565
func asTCPAddress(address string) string {
	// ignore if a prefix already exists
	uriPattern := regexp.MustCompile(`^(\w+:\/\/)[^\s]+$`)
	if uriPattern.MatchString(address) {
		return address
	}

	return "tcp://" + address
}