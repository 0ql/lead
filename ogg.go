package main

import "encoding/binary"

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

type Ogg struct {
	decoder *Decoder
}

/*
Reads the Ogg page information into OggHeader
Further calculates useful information into Added_calc_data
*/
func (ogg *Ogg) ReadOggHeader(pre_skip []byte) (OggHeader, Added_calc_data) {
	dec := &ogg.decoder.buffer

	oggHeader := &OggHeader{
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
	var added_calc_data Added_calc_data = Added_calc_data{}

	if pre_skip != nil {
		added_calc_data.pcm_sample_position = int(binary.LittleEndian.Uint64(oggHeader.granule_position) - uint64(binary.LittleEndian.Uint16(pre_skip)))
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

	ogg.decoder.total_opus_packet_count += added_calc_data.number_of_packets

	return *oggHeader, added_calc_data
}

