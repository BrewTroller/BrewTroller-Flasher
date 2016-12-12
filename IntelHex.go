package main

import (
	"errors"
	"fmt"
	"strconv"
)

const EMPTY_VAL = 0xFF
const START_CODE = 0x3A

//Record type constants
const (
	DATA_REC         = 0x00
	EOF_REC          = 0x01
	EXT_SEG_ADDR_REC = 0x02
	ST_SEG_ADDR_REC  = 0x03
	EXT_LIN_ADDR_REC = 0x04
	ST_LIN_ADDR_REC  = 0x05
)

func parseIntelHex(data []byte) ([]byte, uint64, uint64, error) {

	strData := string(data)
	buf := make([]byte, 0, 8192)

	const MIN_LINE_LEN = 11

	var (
		//bufLen       uint64 = 0
		baseAddr     uint64 = 0
		startSegAddr uint64 = 0
		startLinAddr uint64 = 0
		lineNum      uint64 = 0
		pos          uint64 = 0
	)

	for (pos + MIN_LINE_LEN) < uint64(len(strData)) {
		//Grab a whole line
		if strData[pos] != START_CODE {
			return nil, 0, 0, errors.New(fmt.Sprintf("Line %d does not start with a colon!", lineNum+1))
		} else {
			pos++
			lineNum++
		}

		//Number of data bytes
		dataLen, _ := strconv.ParseUint(strData[pos:pos+2], 16, 8)
		pos += 2
		//Get 16 bit address (Big Endian)
		addrOffset, _ := strconv.ParseUint(strData[pos:pos+4], 16, 16)
		pos += 4
		//Get Record Type
		recType, _ := strconv.ParseUint(strData[pos:pos+2], 16, 8)
		pos += 2
		//Get Data Field
		dataField := make([]byte, dataLen)
		for i := uint64(0); i < dataLen; i++ {
			num, _ := strconv.ParseUint(strData[pos:pos+2], 16, 8)
			dataField[i] = byte(num & 0xFF)
			pos += 2
		}
		//Get check sum
		checksum, _ := strconv.ParseUint(strData[pos:pos+2], 16, 8)
		pos += 2

		calcedCS := calcChecksum(dataLen, addrOffset, recType, dataField)
		if uint8(checksum&0xFF) != calcedCS {
			fmt.Printf("len: %d, offset: %d, rec: %d, data: %v, checksum: %d", dataLen, addrOffset, recType, string(dataField), checksum)
			return nil, 0, 0, errors.New(fmt.Sprintf("Invalid checksum on line %d. Expected %x, got: %x", lineNum, checksum, calcedCS))
		}

		switch recType {
		case DATA_REC:
			absoluteAddr := uint(baseAddr) + uint(addrOffset)
			if (absoluteAddr + uint(dataLen)) >= uint(cap(buf)) {
				//append another slice to extend the buffer
				add := make([]byte, 0, (absoluteAddr+uint(dataLen))*2)
				buf = append(buf, add...)
			}
			//Fill the empty bytes
			if absoluteAddr > uint(len(buf)) {
				for i := uint(len(buf)); i < absoluteAddr; i++ {
					buf[i] = EMPTY_VAL
				}
			}
			//Append the dataField to the buffer
			buf = append(buf, dataField...)
		case EOF_REC:
			if dataLen != 0 {
				return nil, 0, 0, errors.New(fmt.Sprintf("Invalid EOF record on line %d", lineNum))
			}
			return buf, startSegAddr, startLinAddr, nil
		case EXT_SEG_ADDR_REC:
			if dataLen != 2 || addrOffset != 0 {
				return nil, 0, 0, errors.New(fmt.Sprintf("Invalid Extended Segment Address record on line: %d", lineNum))
			}
			baseAddr = uint64((dataField[0]<<8 | dataField[1]) << 4)
		case ST_SEG_ADDR_REC:
			if dataLen != 4 || addrOffset != 0 {
				return nil, 0, 0, errors.New(fmt.Sprintf("Invalid Start Segment Address record on line: %d", lineNum))
			}
			startSegAddr = uint64(dataField[0]<<24) | uint64(dataField[1]<<16) | uint64(dataField[2]<<8) | uint64(dataField[3])
		case EXT_LIN_ADDR_REC:
			if dataLen != 2 || addrOffset != 0 {
				return nil, 0, 0, errors.New(fmt.Sprintf("Invalid Extended Linear Address record on line: %d", lineNum))
			}
			baseAddr = (uint64(dataField[0])<<8 | uint64(dataField[1])) << 16
		case ST_LIN_ADDR_REC:
			if dataLen != 4 || addrOffset != 0 {
				return nil, 0, 0, errors.New(fmt.Sprintf("Invalid Start Linear Address record on line: %d", lineNum))
			}
			startLinAddr = uint64(dataField[0])<<8 | uint64(dataField[1])
		default:
			return nil, 0, 0, errors.New(fmt.Sprintf("Invalid Record Type on line: %d", lineNum))
		}
		if strData[pos] == '\r' {
			pos++
		}
		if strData[pos] == '\n' {
			pos++
		}

	}
	panic("IntelHex Parser exited unexpectedly!")
}

func calcChecksum(dataLen uint64, addrOffset uint64, recType uint64, data []byte) uint8 {
	//sum the dataLen bytes
	var sum uint8 = uint8(dataLen & 0xFF)
	//add the address offset bytes
	sum = sum + uint8(addrOffset&0xFF) + uint8(addrOffset>>8)
	//add the record type
	sum = sum + uint8(recType&0xFF)
	//add the data
	for _, v := range data {
		sum = sum + v
	}
	//Take two's compliment
	sum = (^sum) + 1

	return sum
}
