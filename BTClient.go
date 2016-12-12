package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
    "flag"
    "io/ioutil"
)

type Exit struct{ Code int }

func handleExit() {
	if e := recover(); e != nil {
		if exit, ok := e.(Exit); ok == true {
			log.Println("Exiting...")
			os.Exit(exit.Code)
		}
		panic(e)
	}
}

func panicHandler(f func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if e := recover(); e != nil {
				if exit, ok := e.(Exit); ok == true {
					log.Println("Exiting...")
					os.Exit(exit.Code)
				}
				panic(e)
			}
		}()
		f(w, r)
	}
}

func main() {
	//Handle exit situations
	defer handleExit()

    log.SetPrefix("BTClient: ")
    log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

    var fileName = ""
    var devPort = ""
    flag.StringVar(&fileName, "hex", "", "Prebuilt hex file to upload to device")
    flag.StringVar(&devPort, "port", "", "Serial port of connected BrewTroller")

    flag.Parse()

    if len(fileName) != 0 && len(devPort) != 0 {
        rawHex, err := ioutil.ReadFile(fileName)
        if err != nil {
            log.Println("Error reading hex file: " + err.Error())
            return
        }
        
        d := NewDevice(Port{PortId: devPort})
        err = d.Flash(rawHex)
        if err != nil {
            log.Println("Error flashing device: " + err.Error())
        }
        return
    }

    // Push logging to file
    logfile, err := os.OpenFile("log.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
    if err != nil {
        fmt.Println("Could not open log file!")
    }
    defer logfile.Close()
    log.SetOutput(logfile)
    log.Println("Starting...")

	http.HandleFunc("/", panicHandler(wsHandler))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Println("Could not bind to random port")
		return
	} else {
		defer listener.Close()
		port := strings.Split(listener.Addr().String(), ":")[1]
		fmt.Printf("%s", port)

		herr := http.Serve(listener, nil)
		if herr != nil {
			log.Println("Could not setup http server!")
			return
		}
	}
}


