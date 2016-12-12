package main

import (
	"errors"
	"github.com/tarm/serial"
	"log"
	"runtime"
	"time"
)

type Device struct {
	ComPort         Port
	serialPort      *serial.Port
	serialConfig    serial.Config
	delayMultiplier float32
}

const (
	Resp_STK_OK      byte = 0x10
	Resp_STK_FAILED  byte = 0x11
	Resp_STK_UNKNOWN byte = 0x12
	Resp_STK_NODEV   byte = 0x13
	Resp_STK_INSYNC  byte = 0x14
	Resp_STK_NOSYNC  byte = 0x15

	Sync_CRC_EOP byte = 0x20

	Cmd_STK_GET_SYNC byte = 0x30

	Mem_TYPE_FLASH byte = 0x46 //F

	Cmd_STK_ENTER_PGMMODE byte = 0x50
	Cmd_STK_EXIT_PGMMODE  byte = 0x51
	Cmd_STK_LOAD_ADDR     byte = 0x55

	Cmd_STK_PROG_FLASH    byte = 0x60
	Cmd_STK_PROG_DATA     byte = 0x61
	Cmd_STK_PROG_FUSE     byte = 0x62
	Cmd_STK_PROG_LOCK     byte = 0x63
	Cmd_STK_PROG_PAGE     byte = 0x64
	Cmd_STK_PROG_FUSE_EXT byte = 0x65
)

func NewDevice(p Port) *Device {
	return &Device{ComPort: p, delayMultiplier: 1}
}

func (d *Device) Flash(bin []byte) error {
	p, _, _, err := parseIntelHex([]byte(bin))
	if err != nil {
		return err
	}

	d.serialConfig = serial.Config{Name: d.ComPort.PortId, Baud: 115200, ReadTimeout: time.Microsecond * 4500}
	//if we are on linux, reset the port first
	if runtime.GOOS == "linux" {
		resetLinuxPort(d.ComPort)
	}

	d.serialPort, err = serial.OpenPort(&d.serialConfig)
	if err != nil {
		return err
	}
	defer d.serialPort.Close()

	//try sync at a baud rate of 115200, if not, try at 57600
	err = d.sync()
	if err != nil {
		//Close the port and try again
		d.serialPort.Close()
		d.serialPort = nil
		log.Println("No sync at 115200 Baud, closing port and retrying at 57600...")
		//Wait for the device to reset before retrying
		time.Sleep(time.Second * 3)
		d.serialConfig.Baud = 57600
		d.serialPort, err = serial.OpenPort(&d.serialConfig)
		if err != nil {
			return err
		}
		err = d.sync()
		if err != nil {
			return err
		}
		//Increase the delay multiplier at a slower baud rate
		d.delayMultiplier = 2
	}
	err = d.enterPgmMode()
	if err != nil {
		return err
	}
	log.Println("Beginning Upload...")
	log.Printf("Delay multiplier: %f", d.delayMultiplier)
	
    err = d.upload(p, 256)
    if err != nil {
        return err
    }
	log.Println("Done Upload")
	
    err = d.exitPgmMode()
	if err != nil {
		return err
	}
	return nil
}

func (d *Device) sync() error {
	//started := false
    getSync := []byte{Cmd_STK_GET_SYNC, Sync_CRC_EOP}

	for a := 0; a < 2; a++ {
        log.Println(getSync)
		d.serialPort.Write(getSync)
        time.Sleep(time.Millisecond * 10)
		d.serialPort.Flush()
	}

	for attempt := 0; attempt < 10; attempt++ {
		d.serialPort.Write(getSync)
		buf := make([]byte, 50)
		
        time.Sleep(time.Millisecond * time.Duration(400*d.delayMultiplier))

		_, _ = d.serialPort.Read(buf)

		if buf[0] == Resp_STK_INSYNC {
			log.Println("Got Sync!")
            d.serialPort.Flush()
			return nil
		} else {
            log.Printf("Get Sync: Try %d of %d failed", attempt+1, 10)
            log.Println("Buffer dump....")
            log.Println(buf)
        }
	}
	log.Println("Error syncronizing with bootloader")
	return errors.New("Could not syncronize with bootloader!")
}

func (d *Device)getBootloaderVer() (byte, byte, error) {
    getMVer := []byte{0x41, 0x81, 0x20}
    d.serialPort.Write(getMVer)
    time.Sleep(time.Millisecond * 50)
    buf := make([]byte, 50)
    n, _ := d.serialPort.Read(buf)
    if n != 3 {
        log.Println("Error getting Bootloader Major Version")
        return 0x00, 0x00, errors.New("Unexpected response getting Bootloader Major")
    }
    major := buf[1]

    getMinVer := []byte{0x41, 0x82, 0x20}
    d.serialPort.Write(getMinVer)
    time.Sleep(time.Millisecond * 50)
    buf = make([]byte, 50)
    n, _ = d.serialPort.Read(buf)
    if n != 3 {
        log.Println("Error getting Bootloader Minor Version")
        return 0x00, 0x00, errors.New("Unexpected response getting Bootloader Minor")
    }
    minor := buf[1]

    return major, minor, nil
}

func (d *Device)getSignature() ([]byte, error) {
    getSig := []byte{0x75, 0x20}
    d.serialPort.Write(getSig)
    time.Sleep(time.Millisecond * 50)
    buf := make([]byte, 50)
    n, _ := d.serialPort.Read(buf)
    if n != 5 {
        log.Println("Error getting device signature")
        return nil, errors.New("Unexpected response getting device signature!")
    }
    return buf[1:4], nil
}

func (d *Device)setDeviceParams(params []byte) error {
    //Not Implemented in bootloader
    log.Println("Setting Device Params")
    setParams := []byte{0x42, 0x82, 0x00, 0x00, 0x01, 0x01, 0x01, 0x01, 0x03, 0xFF, 0xFF, 0xFF, 0xFF, 0x01, 0x00, 0x20}
    d.serialPort.Write(setParams)
    buf := make([]byte, 50)
    n, _ := d.serialPort.Read(buf)
    for i := 0; i < n; i++ {
        log.Printf("%x ", buf[i])
    }
    log.Println(" ")
    return nil
}

func (d *Device) enterPgmMode() error {
	enterProg := []byte{0x50, 0x20}
    d.serialPort.Write(enterProg)
    //Delay needs to be higher for windows
    if runtime.GOOS == "windows" {
        time.Sleep(time.Millisecond * 200)
    } else {
        time.Sleep(time.Millisecond * 50)
    }
    buf := make([]byte, 50)
    n, _ := d.serialPort.Read(buf)
    if n != 2 || buf[0] != 0x14 || buf[1] != 0x10 {
        log.Println("Error enterning programming mode")
        return errors.New("Unexepected response when entering programming mode!")
    }
    log.Println("Entered Programming Mode")
    return nil
}

func eraseChip(s *serial.Port) error {
    //Not Implemented in bootloader

    /*fmt.Println("Erase Chip")
    erase := []byte{0x52, 0x20}
    s.Write(erase)
    time.Sleep(time.Millisecond * 500)
    buf = make([]byte, 50)
    n, _ = s.Read(buf)
    for i := 0; i < n; i++ {
        fmt.Printf("%x ", buf[i])
    }
    fmt.Println(" ")*/
    return nil
}

func (d *Device) exitPgmMode() error {
	exitProg := []byte{Cmd_STK_EXIT_PGMMODE, Sync_CRC_EOP}
	if runtime.GOOS == "windows" {
		time.Sleep(time.Millisecond * time.Duration(300*d.delayMultiplier))
		d.serialPort.Flush()
	}
	d.serialPort.Write(exitProg)
	time.Sleep(time.Millisecond * time.Duration(50*d.delayMultiplier))
	buf := make([]byte, 50)
	_, _ = d.serialPort.Read(buf)
	if buf[0] != Resp_STK_INSYNC || buf[1] != Resp_STK_OK {
		log.Println("Error exiting programming mode")
		return errors.New("Unexpected response exiting programming mode!")
	}
	log.Println("Exited Programming Mode")
	return nil
}

func (d *Device) upload(hex []byte, pageSize uint64) error {
	var (
		pageAddr   uint64 = 0
		writeBytes []byte = nil
		useaddr    uint64 = 0
	)
	hexLen := uint64(len(hex))
	for pageAddr < hexLen {
		useaddr = pageAddr >> 1

		err := d.loadAddr(useaddr)
        if err != nil {
            return err
        }

		byteLen := uint64(0)
		if hexLen > pageSize {
			byteLen = pageAddr + pageSize
		} else {
			byteLen = hexLen - 1
		}
		writeBytes = hex[pageAddr:byteLen]
		d.loadPage(writeBytes)
		pageAddr = pageAddr + uint64(len(writeBytes))
		perc := int32((float32(pageAddr) / float32(hexLen)) * 100)
		log.Printf("Uploading: %v%%", perc)

		//Windows needs way more time
		if runtime.GOOS == "windows" {
			time.Sleep(time.Millisecond * time.Duration(30*d.delayMultiplier))
		} else {
			time.Sleep(time.Millisecond * 3)
		}
	}
    return nil
}

func (d *Device) loadAddr(addr uint64) error {
	//fmt.Println("Load Address")
    lowAddr := byte(addr & 0xFF)
    highAddr := byte((addr >> 8) & 0xFF)
    addrLoad := []byte{0x55, lowAddr, highAddr, 0x20}
    d.serialPort.Write(addrLoad)
    time.Sleep(time.Microsecond * 25)
    buf := make([]byte, 50)
    _, _ = d.serialPort.Read(buf)
    return nil
}

func (d *Device) loadPage(writeBytes []byte) {
	//fmt.Println("Load Page")
    lenLow := byte(len(writeBytes) & 0xFF)
    lenHigh := byte((len(writeBytes) >> 8) & 0xFF)
    pageLoad := []byte{0x64, lenHigh, lenLow, 0x46}
    pageLoad = append(pageLoad, writeBytes...)
    pageLoad = append(pageLoad, 0x20)
    d.serialPort.Write(pageLoad)
    time.Sleep(time.Microsecond * 4500)
    buf := make([]byte, 50)
    _, _ = d.serialPort.Read(buf)
}
