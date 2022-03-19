package OggOpusMeta

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

type OggHeader struct {
	capture_pattern          []byte // Ascii "OggS"
	stream_structure_version []byte // Version must be 0
	header_type              []byte // 0x01 Continuation | 0x02 BOS | 0x04 EOS
	granule_position         []byte
	bitstream_serial_number  []byte
	page_sequence_number     []byte
	checksum                 []byte
	page_segments            []byte // number of segments (aka. Opus Packets) in the lacing table
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

type OggOpusMeta struct {
	opusHead                OpusHead
	opusTags                OpusTags
	total_opus_packet_count int
	buffer                  ByteReader
}

func CreateDecoder(file []byte) OggOpusMeta {
	return OggOpusMeta{
		opusHead:                OpusHead{},
		opusTags:                OpusTags{},
		total_opus_packet_count: 0,
		buffer: ByteReader{
			Position: 0,
			Buf:      file,
		},
	}
}

/**
Reads the Ogg page information into OggHeader
Further calculates useful information into Added_calc_data
*/
func (meta *OggOpusMeta) ReadOggHeader(pre_skip []byte) (OggHeader, Added_calc_data) {
	dec := meta.buffer

	oggHeader := OggHeader{
		capture_pattern:          dec.Read(4),
		stream_structure_version: dec.Read(1),
		header_type:              dec.Read(1),
		granule_position:         dec.Read(8),
		bitstream_serial_number:  dec.Read(4),
		page_sequence_number:     dec.Read(4),
		checksum:                 dec.Read(4),
		page_segments:            dec.Read(1),
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

	meta.total_opus_packet_count += added_calc_data.number_of_packets

	return oggHeader, added_calc_data
}

func (meta *OggOpusMeta) ReadOpusHead() OpusHead {
	/**
	1) [vendor_length] = read an unsigned integer of 32 bits
	2) [vendor_string] = read a UTF-8 vector as [vendor_length] octets
	3) [user_comment_list_length] = read an unsigned integer of 32 bits
	4) iterate [user_comment_list_length] times {
	5) 		[length] = read an unsigned integer of 32 bits
	6)		this iteration's user comment = read a UTF-8 vector as [length] octets
		 }
	*/
	dec := meta.buffer

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

func (meta *OggOpusMeta) ReadOpusTags() OpusTags {
	dec := meta.buffer
	opusTags := OpusTags{}
	opusTags.opus_tags = dec.Read(8)
	opusTags.vendor_string_length = dec.Read(4)
	opusTags.vendor_string = dec.Read(int(binary.LittleEndian.Uint32(opusTags.vendor_string_length)))
	opusTags.metadata_string_length = dec.Read(4)
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
func (meta *OggOpusMeta) ReadOpusPackets(data Added_calc_data) [][]byte {
	dec := meta.buffer

	packets := [][]byte{}
	for _, length := range data.packet_lengths {
		packet := dec.Read(length)
		// decode to pcm whatever...
		packets = append(packets, packet)
	}

	return packets
}

func example() {
	f, err := os.ReadFile("./example.opus")
	if err != nil {
		panic(err)
	}

	decoder := CreateDecoder(f)

	oggHeader1, _ := decoder.ReadOggHeader(nil)
	opusHead := decoder.ReadOpusHead()
	oggHeader2, _ := decoder.ReadOggHeader(nil)
	opusTags := decoder.ReadOpusTags()
	oggHeader3, addedData1 := decoder.ReadOggHeader(opusHead.pre_skip)
	decoder.ReadOpusPackets(addedData1)
	oggHeader4, addedData2 := decoder.ReadOggHeader(opusHead.pre_skip)
	decoder.ReadOpusPackets(addedData2)

	eos := false
	for eos == false {
		oggHeader, data := decoder.ReadOggHeader(opusHead.pre_skip)
		if oggHeader.header_type[0] == 4 { // check for end of stream (EOS)
			eos = true
		}
		decoder.ReadOpusPackets(data)
	}

	fmt.Printf("%+v\n", oggHeader1)
	fmt.Printf("%+v\n", opusHead)
	fmt.Printf("%+v\n", oggHeader2)
	fmt.Printf("%+v\n", opusTags)
	fmt.Printf("%+v\n", oggHeader3)
	fmt.Printf("%+v\n", addedData1)
	fmt.Printf("%+v\n", oggHeader4)
	fmt.Printf("%+v\n", addedData2)
	fmt.Println(decoder.total_opus_packet_count)
}

func main() {
	example()
}
