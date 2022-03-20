package main

import (
	"encoding/binary"
	"fmt"
	"math/bits"
)

// Variable-Size Integer
type VINT struct {
	Width int
	Data  []byte
	Value uint64
}

type EBML_Element struct {
	ID   []byte
	Size []byte
	Data []byte
}

type EBML_Body_ELements struct {
	ID   []byte
	Size []byte
}

type EBML_Header_Elements struct {
	ID                    []byte
	Size                  []byte
	EBML_Version          EBML_Element
	EBML_Read_Version     EBML_Element
	EBML_Max_ID_Length    EBML_Element
	EBML_Max_Size_Length  EBML_Element
	Doc_Type              EBML_Element
	Doc_Type_Version      EBML_Element
	Doc_Type_Read_Version EBML_Element
}

type EBML_Root struct {
	EBML_Header EBML_Header_Elements
	EBML_Body   EBML_Body_ELements
}

type Webm struct {
	decoder *Decoder
}

// Reads one byte from the buffer any more belonging to the VINT
func (webm *Webm) ReadVINT() VINT {
	dec := &webm.decoder.buffer

	b := dec.Read(1)
	width := bits.LeadingZeros8(uint8(b[0]))
	andByte := byte(1)
	andByte = andByte << uint(7-width)

	b[0] = andByte ^ b[0]

	for i := width; i <= width*7+7; i++ {
		if i%8 == 0 {
			b = append(b, dec.Read(1)...)
		}
	}

	y := make([]byte, 8-len(b))
	b = append(y, b...)

	value := binary.BigEndian.Uint64(b)

	return VINT{
		Width: width,
		Value: value,
	}
}

func (webm *Webm) ReadEBML() {
	dec := &webm.decoder.buffer

	ebml := EBML_Root{
		EBML_Header: EBML_Header_Elements{
			ID:   dec.Read(4), // 0x1A45DFA3
			Size: dec.Read(1), // 0x9F
			EBML_Version: EBML_Element{
				ID:   dec.Read(2), // 0x4286
				Size: dec.Read(1), // 0x81
				Data: dec.Read(1), // 0x01 (Must be Version 1)
			},
			EBML_Read_Version: EBML_Element{
				ID:   dec.Read(2), // 0x42F7
				Size: dec.Read(1), // 0x81
				Data: dec.Read(1), // 0x01
			},
			EBML_Max_ID_Length: EBML_Element{
				ID:   dec.Read(2), // 0x42F2
				Size: dec.Read(1), // 0x81
				Data: dec.Read(1), // 0x04 (4 or less in Matroska)
			},
			EBML_Max_Size_Length: EBML_Element{
				ID:   dec.Read(2), // 0x42F3
				Size: dec.Read(1), // 0x81
				Data: dec.Read(1), // 0x08 (8 or less in Matroska)
			},
			Doc_Type: EBML_Element{
				ID:   dec.Read(2), // 0x4282
				Size: dec.Read(1), // 0x84
				Data: dec.Read(4), // 0x7765626D (Ascii "webm")
			},
			Doc_Type_Version: EBML_Element{
				ID:   dec.Read(2), // 0x4287
				Size: dec.Read(1), // 0x81
				Data: dec.Read(1), // 0x04
			},
			Doc_Type_Read_Version: EBML_Element{
				ID:   dec.Read(2), // 0x4285
				Size: dec.Read(1), // 0x81
				Data: dec.Read(1), // 0x02
			},
		},
		EBML_Body: EBML_Body_ELements{
			ID: dec.Read(4), // 0x18538067
		},
	}
	fmt.Println(webm.ReadVINT())

	fmt.Printf("%+v\n", ebml)
}
