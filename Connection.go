package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
)

const (
	maxMessageSize = 1024 * 1024 * 200 //200KB message size limit
	// The 200KB size limit should be sufficient as the compiled code size for our chip < 132KB

	MTYPE_DEV_FIND  = "1"
	MTYPE_FLASH_HEX = "2"
)

type client struct {
	ws *websocket.Conn
}

func (c *client) wsReader() {
	defer c.ws.Close()
	defer panic(Exit{0})

	c.ws.SetReadLimit(maxMessageSize)

	for {
		message := make(map[string]string)
		err := c.ws.ReadJSON(&message)
		if err != nil {
			//If we got an error somehting went wrong
			//  Likely the parent process orphaned us, so we should panic out
			log.Println("Error Encountered, closing connection...")
			break
		}
		if val, ok := message["type"]; ok {

			switch val {
			case MTYPE_DEV_FIND:
				d := scanForDevices()
				c.writeJSON(d)
				break
			case MTYPE_FLASH_HEX:
				if payload, ok := message["payload"]; ok {
					if dev, have := message["device"]; have {
                        p := Port{PortId: dev}
                        d := NewDevice(p)
						e := d.Flash([]byte(payload))
						resp := make(map[string]string)
						if e != nil {
							resp["flash"] = e.Error()
						} else {
							resp["flash"] = "complete"
						}
						c.writeJSON(resp)
					}
				}
				break
			}

		} else {
			continue
		}

	}
}

func (c *client) writeJSON(v interface{}) error {
	return c.ws.WriteJSON(v)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  maxMessageSize,
	WriteBufferSize: maxMessageSize,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func wsHandler(rw http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" {
		http.Error(rw, "Method not allowed", 405)
		return
	}

	conn, err := upgrader.Upgrade(rw, req, nil)
	if err != nil {
		//If there is an error upgrading, there is nothing we can do
		fmt.Println(err)
		return
	}

	c := &client{
		ws: conn,
	}
	c.wsReader()
}
