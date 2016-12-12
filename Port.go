package main

import (
	"runtime"
    "path/filepath"
    "os/exec"
    "regexp"
    "strings"
    "github.com/tarm/serial"
    "log"
    "time"
)

const (
    LINUX_SERIAL   = "/dev/ttyUSB*"
    MAC_SERIAL     = "/dev/tty.usbserial-*"
    WINDOWS_SERIAL = "wmic path Win32_PnPEntity Get Name"
)

type Port struct {
	PortId string
	Status string
}

func scanForDevices() []Port {
	switch runtime.GOOS {
	case "darwin":
		return getPosixDevices(MAC_SERIAL)
	case "windows":
		return getWindowsDevices()
	case "linux":
		return getPosixDevices(LINUX_SERIAL)
	}
	return nil
}

func getPosixDevices(path string) []Port {
	ports, _ := filepath.Glob(path)
	return testPorts(ports)
}

func getWindowsDevices() []Port {
	//get the list from wmic
	getPorts := exec.Command("cmd", "/C", WINDOWS_SERIAL)
	raw, err := getPorts.CombinedOutput()
	if err != nil {
		return nil
	}
	list := strings.Split(string(raw), "\r\n")
	ports := make([]string, 0, 10)
	regex, _ := regexp.Compile("(COM[0-9]+)")
	for _, v := range list {
		matches := regex.FindAllString(v, 1)
		if len(matches) == 1 {
			ports = append(ports, matches[0])
		}
	}
	return testPorts(ports)
}

func resetLinuxPort(p Port) {
	cmd := exec.Command("stty", "hupcl", "-F", p.PortId)
	_ = cmd.Run()
}

func testPorts(p []string) []Port {
	d := make([]Port, 0, len(p))
	buf := make([]byte, 128)
	for _, port := range p {
		//try to open the port and read back its status string
		c := &serial.Config{Name: port, Baud: 115200, ReadTimeout: time.Second * 5}
		s, err := serial.OpenPort(c)
		if err != nil {
			continue
		}
		defer s.Close()
		if runtime.GOOS == "linux" {
			defer resetLinuxPort(Port{PortId: port})
		}
		time.Sleep(time.Millisecond * 2500)
		n, _ := s.Read(buf)
		if n > 0 {
			dat := string(buf[:n])
			log.Println("Device " + port + " returned: " + dat)
			matcher, _ := regexp.Compile("(SYS|838983)\t(VER|866982)")
			if match := matcher.Find(buf[:n]); match != nil {
				d = append(d, Port{PortId: port, Status: dat})
			}
			continue
		}
	}
	return d
}
