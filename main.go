package main

import (
	"encoding/binary"
	"fmt"
	"os"
)

type Decoder struct {
	Position int
	Buf      []byte
}

func (dec *Decoder) Read(n int) []byte {
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

type OggHeader struct {
	capture_pattern          []byte
	stream_structure_version []byte
	header_type              []byte
	granule_position         []byte
	bitstream_serial_number  []byte
	page_sequence_number     []byte
	checksum                 []byte
	page_segments            []byte
	lacing_table             []byte
}

type Added_calc_data struct {
	pcm_sample_position int
	playback_time       int
	total_body_length   int
	number_of_packets   int
	packet_lengths      []int
}

type OpusHead struct {
	opus_head                      []byte
	version                        []byte
	channel_count                  []byte
	pre_skip                       []byte
	original_input_sample_rate     []byte
	output_gain                    []byte
	channel_map                    []byte
	optional_channel_mapping_table []byte
}

type OpusTags struct {
	opus_tags              []byte
	vendor_string_length   []byte
	vendor_string          []byte
	metadata_string_length []byte
	metadata_strings       []byte
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

var dec Decoder
var totalPackets int = 0

func readOggHeader(pre_skip []byte) (OggHeader, Added_calc_data) {
	oggHeader := OggHeader{
		capture_pattern:          dec.Read(4), // Ascii "OggS"
		stream_structure_version: dec.Read(1), // Version must be 0
		header_type:              dec.Read(1), // 0x01 Continuation | 0x02 BOS | 0x04 EOS
		granule_position:         dec.Read(8),
		bitstream_serial_number:  dec.Read(4),
		page_sequence_number:     dec.Read(4),
		checksum:                 dec.Read(4),
		page_segments:            dec.Read(1), // number of segments in the lacing table
	}

	oggHeader.lacing_table = dec.Read(int(oggHeader.page_segments[0]))
	var added_calc_data Added_calc_data

	if pre_skip != nil {
		added_calc_data = Added_calc_data{
			pcm_sample_position: int(binary.LittleEndian.Uint64(oggHeader.granule_position) - uint64(binary.LittleEndian.Uint16(pre_skip))),
		}

		added_calc_data.playback_time = added_calc_data.pcm_sample_position / 48000
	}

	i := 0
	packetLengths := []int{}
	for i < len(oggHeader.lacing_table) {
		packetLength := 0
		packetLength += int(oggHeader.lacing_table[i])
		for oggHeader.lacing_table[i] == 255 {
			i++
			packetLength += int(oggHeader.lacing_table[i])
		}
		packetLengths = append(packetLengths, packetLength)
		i++
	}
	added_calc_data.total_body_length = sum(packetLengths)
	added_calc_data.number_of_packets = len(packetLengths)
	added_calc_data.packet_lengths = packetLengths

	totalPackets += added_calc_data.number_of_packets

	return oggHeader, added_calc_data
}

func readOpusHead() OpusHead {
	opusHead := OpusHead{
		opus_head:                  dec.Read(8), // Ascii "OpusHead"
		version:                    dec.Read(1), // 0x01 for this spec
		channel_count:              dec.Read(1),
		pre_skip:                   dec.Read(2), // Pre-skip (16 bits unsigned, little endian)
		original_input_sample_rate: dec.Read(4), // Input sample rate (32 bits unsigned, little endian): informational only
		output_gain:                dec.Read(2), // Output gain (16 bits, little endian, signed Q7.8 in dB) to apply when decoding
		channel_map:                dec.Read(1), // usually 0
		/**
		- Channel mapping family (8 bits unsigned)
			- 0 = one stream: mono or L,R stereo
			- 1 = channels in vorbis spec order: mono or L,R stereo or ... or FL,C,FR,RL,RR,LFE, ...
			- 2..254 = reserved (treat as 255)
			- 255 = no defined channel meaning

		- If channel mapping family > 0
			- Stream count 'N' (8 bits unsigned): MUST be > 0
			- Two-channel stream count 'M' (8 bits unsigned): MUST satisfy M <= N, M+N <= 255
			- Channel mapping (8*c bits)
				-- one stream index (8 bits unsigned) per channel (255 means silent throughout the file)
		*/
		optional_channel_mapping_table: nil,
	}
	return opusHead
}

func readOpusTags() OpusTags {
	/**
	  1) [vendor_length] = read an unsigned integer of 32 bits
	  2) [vendor_string] = read a UTF-8 vector as [vendor_length] octets
	  3) [user_comment_list_length] = read an unsigned integer of 32 bits

	  4) iterate [user_comment_list_length] times {
	       5) [length] = read an unsigned integer of 32 bits
	       6) this iteration's user comment = read a UTF-8 vector as [length] octets
			 }
	*/
	opusTags := OpusTags{}
	opusTags.opus_tags = dec.Read(8)            // Ascii "OpusTags"
	opusTags.vendor_string_length = dec.Read(4) // (always present) 4-byte little-endian length field, followed by length bytes of UTF-8 vendor string.
	opusTags.vendor_string = dec.Read(int(binary.LittleEndian.Uint32(opusTags.vendor_string_length)))
	opusTags.metadata_string_length = dec.Read(4) // 4-byte little-endian string count
	n := binary.LittleEndian.Uint32(opusTags.metadata_string_length)
	for i := uint32(0); i < n; i++ {
		length := dec.Read(4)
		opusTags.metadata_strings = append(opusTags.metadata_strings, dec.Read(int(binary.LittleEndian.Uint32(length)))...)
	}
	return opusTags
}

func readOpusPackets(data Added_calc_data) {
	tocs := []byte{}
	for _, length := range data.packet_lengths {
		packet := dec.Read(length)
		// decode to pcm whatever...
		tocs = append(tocs, packet[0])
	}
	fmt.Println(tocs)
}

func main() {
	f, err := os.ReadFile("./gimme.opus")
	check(err)

	dec = Decoder{
		Position: 0,
		Buf:      f,
	}

	oggHeader1, _ := readOggHeader(nil)
	opusHead := readOpusHead()
	oggHeader2, _ := readOggHeader(nil)
	opusTags := readOpusTags()
	oggHeader3, addedData1 := readOggHeader(opusHead.pre_skip)
	readOpusPackets(addedData1)
	oggHeader4, addedData2 := readOggHeader(opusHead.pre_skip)
	readOpusPackets(addedData2)

	eos := false
	for eos == false {
		oggHeader, data := readOggHeader(opusHead.pre_skip)
		if oggHeader.header_type[0] == 4 { // check for end of stream (EOS)
			eos = true
		} 
		readOpusPackets(data)
	}

	fmt.Printf("%+v\n", oggHeader1)
	fmt.Printf("%+v\n", opusHead)
	fmt.Printf("%+v\n", oggHeader2)
	fmt.Printf("%+v\n", opusTags)
	fmt.Printf("%+v\n", oggHeader3)
	fmt.Printf("%+v\n", addedData1)
	fmt.Printf("%+v\n", oggHeader4)
	fmt.Printf("%+v\n", addedData2)
	fmt.Println(totalPackets)
}
