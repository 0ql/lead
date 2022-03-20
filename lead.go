package main

import (
	"encoding/binary"
	"fmt"
	"os"
)

type ByteReader struct {
	Position int
	Buf      []byte
}

func (dec *ByteReader) Read(n int) []byte {
	dec.Position += n
	return dec.Buf[dec.Position-n : dec.Position]
}

func sum(array []int) int {
	result := 0
	for _, v := range array {
		result += v
	}
	return result
}

type OpusHead struct {
	opus_head                      []byte // Ascii "OpusHead"
	version                        []byte // 0x01 for this spec
	channel_count                  []byte
	pre_skip                       []byte // Pre-skip (16 bits unsigned, little endian)
	original_input_sample_rate     []byte // Input sample rate (32 bits unsigned, little endian): informational only
	output_gain                    []byte // Output gain (16 bits, little endian, signed Q7.8 in dB) to apply when decoding
	channel_map                    []byte // usually 0
	optional_channel_mapping_table []byte // I'm ignoring it as it's usually empty
}

type OpusTags struct {
	opus_tags              []byte // Ascii "OpusTags"
	vendor_string_length   []byte // (always present) 4-byte little-endian length field, followed by length bytes of UTF-8 vendor string
	vendor_string          []byte
	metadata_string_length []byte // 4-byte little-endian string count
	metadata_strings       []byte
}

type Decoder struct {
	opusHead                OpusHead
	opusTags                OpusTags
	total_opus_packet_count int
	buffer                  ByteReader
	ogg                     Ogg
	webm                    Webm
}

func CreateDecoder(file []byte) *Decoder {
	dec := &Decoder{
		opusHead:                OpusHead{},
		opusTags:                OpusTags{},
		ogg:                     Ogg{},
		webm:                    Webm{},
		total_opus_packet_count: 0,
		buffer: ByteReader{
			Position: 0,
			Buf:      file,
		},
	}
	dec.ogg.decoder = dec
	dec.webm.decoder = dec
	return dec
}

func (meta *Decoder) ReadOpusHead() OpusHead {
	/**
	1) [vendor_length] = read an unsigned integer of 32 bits
	2) [vendor_string] = read a UTF-8 vector as [vendor_length] octets
	3) [user_comment_list_length] = read an unsigned integer of 32 bits
	4) iterate [user_comment_list_length] times {
	5) 		[length] = read an unsigned integer of 32 bits
	6)		this iteration's user comment = read a UTF-8 vector as [length] octets
		 }
	*/
	dec := &meta.buffer

	opusHead := OpusHead{
		opus_head:                      dec.Read(8),
		version:                        dec.Read(1),
		channel_count:                  dec.Read(1),
		pre_skip:                       dec.Read(2),
		original_input_sample_rate:     dec.Read(4),
		output_gain:                    dec.Read(2),
		channel_map:                    dec.Read(1),
		optional_channel_mapping_table: nil,
	}
	return opusHead
}

func (meta *Decoder) ReadOpusTags() OpusTags {
	dec := &meta.buffer

	opusTags := OpusTags{}
	opusTags.opus_tags = dec.Read(8)
	opusTags.vendor_string_length = dec.Read(4)
	opusTags.vendor_string = dec.Read(int(binary.LittleEndian.Uint32(opusTags.vendor_string_length)))
	opusTags.metadata_string_length = dec.Read(4)
	fmt.Printf("%+v\n", opusTags)
	n := binary.LittleEndian.Uint32(opusTags.metadata_string_length)
	for i := uint32(0); i < n; i++ {
		length := dec.Read(4)
		opusTags.metadata_strings = append(opusTags.metadata_strings, dec.Read(int(binary.LittleEndian.Uint32(length)))...)
	}
	return opusTags
}

/**
Returns an array of raw Opus Packets
*/
func (meta *Decoder) ReadOpusPackets(data Added_calc_data) [][]byte {
	dec := &meta.buffer

	packets := [][]byte{}
	for _, length := range data.packet_lengths {
		packet := dec.Read(length)
		// decode to pcm whatever...
		packets = append(packets, packet)
	}

	return packets
}

func exampleOgg() {
	f, err := os.ReadFile("./example.opus")
	if err != nil {
		panic(err)
	}

	decoder := CreateDecoder(f)

	oggHeader1, _ := decoder.ogg.ReadOggHeader(nil)
	fmt.Printf("\nOggHeader: %+v\n", oggHeader1)

	opusHead := decoder.ReadOpusHead()
	fmt.Printf("\nOpusHead: %+v\n", opusHead)

	oggHeader2, _ := decoder.ogg.ReadOggHeader(nil)
	fmt.Printf("\nOggHeader: %+v\n", oggHeader2)

	opusTags := decoder.ReadOpusTags()
	fmt.Printf("\nOpusTags: %+v\n", opusTags)

	oggHeader3, addedData1 := decoder.ogg.ReadOggHeader(opusHead.pre_skip)
	fmt.Printf("\nOggHeader: %+v\n", oggHeader3)
	fmt.Printf("Added Data: %+v\n", addedData1)

	data := decoder.ReadOpusPackets(addedData1)
	fmt.Println("Opus Packets: ", data)

	eos := false
	for eos == false {
		oggHeader, data := decoder.ogg.ReadOggHeader(opusHead.pre_skip)
		if oggHeader.header_type[0] == 4 { // check for end of stream (EOS)
			eos = true
		}
		decoder.ReadOpusPackets(data)
	}

	fmt.Println(decoder.total_opus_packet_count)
}

func exampleWebM() {
	file, err := os.ReadFile("./test-opus.webm")
	if err != nil {
		panic(err)
	}

	decoder := CreateDecoder(file)

	decoder.webm.ReadEBML()

	fmt.Println(file[:100])
}

func main() {
	exampleWebM()
}
