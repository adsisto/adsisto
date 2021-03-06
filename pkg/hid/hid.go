/*
 * Adsisto
 * Copyright (c) 2019 Andrew Ying
 *
 * This program is free software: you can redistribute it and/or modify it under
 * the terms of version 3 of the GNU General Public License as published by the
 * Free Software Foundation. In addition, this program is also subject to certain
 * additional terms available at <SUPPLEMENT.md>.
 *
 * This program is distributed in the hope that it will be useful, but WITHOUT ANY
 * WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR
 * A PARTICULAR PURPOSE.  See the GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License along with
 * this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package hid

import (
	"encoding/hex"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"os"
	"strings"
)

// Stream is a instance of HID device.
type Stream struct {
	Device string
}

// StreamMessage is a instance of the message to be streamed to the HID device.
type StreamMessage struct {
	Key   string
	Ctrl  bool
	Shift bool
	Alt   bool
	Meta  bool
}

// WebsocketHandler sets up a WebSocket instance for receiving keystrokes events
// from the client.
func (s *Stream) WebsocketHandler(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		panic(err)
	}

	file, err := os.Create(s.Device)
	if err != nil {
		panic(err)
	}

	defer ws.Close()
	defer file.Close()

	for {
		message := StreamMessage{}
		err := ws.ReadJSON(message)
		if err != nil {
			log.Print(err)
		}

		message.ParseMessage()
		if message.Key != "" {
			bytes := message.GenerateHID()
			bytesEncoded := hex.EncodeToString(bytes[:])
			bytesEncoded = strings.Replace(bytesEncoded, "0x", "\\x", -1)

			command := fmt.Sprintf("printf \"%%b\" '%v' | hid-ops keyboard", bytesEncoded)
			_, err = file.Write([]byte(command))
			if err != nil {
				log.Print(err)
			}
		}
	}
}
