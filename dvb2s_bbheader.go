package dvb2s

const (
	GenericStreamPacketized = uint8(0x00)
	GenericStreamContinuous = uint8(0x40)
	TransportStream         = uint8(0xc0)

	MultipleInputStream = uint8(0x00)
	SingleInputStream   = uint8(0x20)

	AdaptiveCodingModulation = uint8(0x00)
	ConstantCodingModulation = uint8(0x10)

	IStreamSyncIndicatorNo  = uint8(0x00)
	IStreamSyncIndicatorYes = uint8(0x08)

	NullPacketDeletionNo  = uint8(0x00)
	NullPacketDeletionYes = uint8(0x04)

	TransmissionRolloffFactor035 = uint8(0x00)
	TransmissionRolloffFactor025 = uint8(0x01)
	TransmissionRolloffFactor020 = uint8(0x02)
)

type bbHeader struct {
	bytes                         [10]uint8
	matype1                       []uint8
	matype2                       []uint8
	userPacketLength              []uint8
	dataFieldLength               []uint8
	userPacketSyncByte            []uint8
	dataFieldToUserPacketDistance []uint8
	crc8                          []uint8
	bitstream                     [80]bool
}

func newBbHeader() *bbHeader {
	var h bbHeader

	h.matype1 = h.bytes[0:1]
	h.matype2 = h.bytes[1:2]
	h.userPacketLength = h.bytes[2:4]
	h.dataFieldLength = h.bytes[4:6]
	h.userPacketSyncByte = h.bytes[6:7]
	h.dataFieldToUserPacketDistance = h.bytes[7:9]
	h.crc8 = h.bytes[9:10]

	h.matype1[0] = TransportStream | SingleInputStream | ConstantCodingModulation | TransmissionRolloffFactor035
	h.matype2[0] = uint8(0x00)

	upl := 188 * 8
	h.userPacketLength[0] = uint8(upl >> 8)
	h.userPacketLength[1] = uint8(upl)

	dfl := 48408 - len(h.bitstream)
	h.dataFieldLength[0] = uint8(dfl >> 8)
	h.dataFieldLength[1] = uint8(dfl)

	h.userPacketSyncByte[0] = uint8(0x47)

	h.dataFieldToUserPacketDistance[0] = uint8(0x00)
	h.dataFieldToUserPacketDistance[1] = uint8(0x00)

	j := 0
	for _, b := range h.bytes {
		for i := 0; i < 8; i++ {
			h.bitstream[j] = (b & uint8(0x80)) > 0
			b <<= 1
			j++
		}
	}

	h.crc8[0] = h.crc8Encode(h.bitstream[:len(h.bitstream)-8])

	for i, sr := len(h.bitstream)-8, h.crc8[0]; i < len(h.bitstream); i, sr = i+1, sr>>1 {
		h.bitstream[i] = (sr & 0x01) > 0
	}

	return &h
}

func (h *bbHeader) crc8Encode(data []bool) uint8 {
	// poly := uint8(0x57)
	// sr := uint8(0x00)

	// for _, value := range data {
	// 	if (sr&0x01 > 0) != value {
	// 		sr = ((sr ^ poly) >> 1) | 0x80
	// 	} else {
	// 		sr >>= 1
	// 	}
	// }

	poly := uint8(0xab)
	sr := uint8(0x00)

	for _, value := range data {
		swap := (sr&0x01 > 0) != value
		sr >>= 1
		if swap {
			sr ^= poly
		}
	}

	return sr
}

func (h *bbHeader) getUserPacketLength() int {
	return int((uint16(h.userPacketLength[0]) << 8) | uint16(h.userPacketLength[1]))
}

func (h *bbHeader) getDataFieldLength() int {
	return int((uint16(h.dataFieldLength[0]) << 8) | uint16(h.dataFieldLength[1]))
}
