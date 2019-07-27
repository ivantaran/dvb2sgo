package dvb2s

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type dvb2s struct {
	modcod              int
	fecFrameType        int
	oversampling        int
	ldpcQFactor         int
	bitsPerPlSymbol     int
	interpolateByRepeat bool
	bbFrame             []bool
	bchBlock            []bool
	bchFec              []bool
	ldpcFec             []bool
	fecFrame            []bool
	plFrame             []complex128 // TODO: combine plFrame with plHeader
	plHeader            []complex128
	outFrame            []complex128
	firFilter           *fir
	bbHeader            *bbHeader
}

func newDvb2s(fecFrameType string, oversampling int, interpolateByRepeat bool) *dvb2s {
	var d dvb2s

	fecFrameSize := fecFramesizeMap[fecFrameType]

	// TODO : repair this
	// enum {
	// 	QPSK3_4 = 7
	// };

	// enum {
	// 	PL_NORMAL = 0,
	// 	PL_SMALL = 1,
	// 	PL_NORMAL_P = 2,
	// 	PL_SMALL_P = 3,
	// };

	d.fecFrameType = 0
	d.modcod = 7

	d.oversampling = oversampling
	bchBlockSize := fecFrameSize * 3 / 4
	ldpcFecSize := fecFrameSize - bchBlockSize
	bchTErrorCorrection := 12 // TODO: select from table 5a, 5b
	bchFecSize := bchTErrorCorrection * (bchPolyNLength - 1)
	bbFrameSize := bchBlockSize - bchFecSize

	d.ldpcQFactor = ldpcFecSize / ldpcBlockSize
	d.bitsPerPlSymbol = 2

	plFrameSize := fecFrameSize / d.bitsPerPlSymbol
	outFrameSize := plFrameSize * d.oversampling

	d.fecFrame = make([]bool, fecFrameSize)
	d.bbFrame = d.fecFrame[:bbFrameSize]
	d.bchBlock = d.fecFrame[:bchBlockSize]
	d.bchFec = d.fecFrame[bbFrameSize:bchBlockSize]
	d.ldpcFec = d.fecFrame[bchBlockSize:]
	d.plFrame = make([]complex128, plFrameSize)
	d.plHeader = make([]complex128, slotSize)
	d.outFrame = make([]complex128, outFrameSize)

	switch oversampling {
	case 2:
		// d.firFilter = newFir(firRrc2x035Table)
		d.firFilter = newFir(firRrc2x035BigTable)
	case 4:
		d.firFilter = newFir(firRrc4x035Table)
	default:
		panic("unknown oversampling\n")
	}

	d.interpolateByRepeat = interpolateByRepeat

	d.bbHeader = newBbHeader()

	return &d
}

func (d *dvb2s) LoadInputData(fileName string) error {

	file, err := os.Open(fileName)

	if err != nil {
		return err
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)

	var i int
	for scanner.Scan() {
		value, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
		if err != nil {
			fmt.Printf("last line is: \"%s\"\n", scanner.Text())
			break
		}
		d.bbFrame[i] = value > 0
		i++
		if i == len(d.bbFrame) {
			break
		}
	}

	fmt.Printf("loaded data size: %d\n", i)

	return nil
}

func (d *dvb2s) bbFrameScramble() {
	init := 0x4a80

	sr := init
	for i := range d.bbFrame {
		fb := ((sr << 14) ^ (sr << 13)) & 0x4000
		d.bbFrame[i] = d.bbFrame[i] != (fb > 0)
		sr = ((sr >> 1) & 0x3fff) | fb
	}
}

func (d *dvb2s) bchPolymul(a [bchPolyNLength]bool, b []bool, lenb int, r []bool) int {

	var len int

	for i := 0; i < bchPolyNLength+lenb; i++ {
		r[i] = false
	}

	for j := 0; j < lenb; j++ {
		for i := 0; i < bchPolyNLength; i++ {
			r[i+j] = r[i+j] != (a[i] && b[j])
		}
	}

	for i := 0; i < bchPolyNLength+lenb; i++ {
		if r[i] {
			len = i
		}
	}

	return len + 1
}

func (d *dvb2s) bchInit(gpoly []bool) int {

	gpoly1 := make([]bool, len(gpoly))

	ll := d.bchPolymul(bchPolyN[0], bchPolyN[1][:], bchPolyNLength, gpoly1)

	p1 := gpoly1
	p2 := gpoly

	for i := 2; i < len(bchPolyN); i++ { //TODO: replace len(bchPolyN) to parameter = [8, 10, 12]
		ll = d.bchPolymul(bchPolyN[i], p1, ll, p2)
		p1, p2 = p2, p1
	}

	for i := 0; i < ll; i++ {
		gpoly[i] = gpoly1[ll-i-1]
	}

	return ll
}

func (d *dvb2s) bchEncode(gpoly []bool) {

	for i := range d.bchFec {
		d.bchFec[i] = false
	}

	len := len(d.bchFec)
	for _, value := range d.bbFrame {
		fb := d.bchFec[0] != value
		for i := 0; i < len-1; i++ {
			d.bchFec[i] = (gpoly[i+1] && fb) != d.bchFec[i+1] // TODO: length gpoly must be 192, not 193
		}
		d.bchFec[len-1] = fb
	}

}

func (d *dvb2s) ldpcEncode() {
	for j, row := range ldpcTable3_4 {
		for i := 0; i < ldpcBlockSize; i++ {
			for _, value := range row {
				addr := (int(value) + i*d.ldpcQFactor) % len(d.ldpcFec)
				d.ldpcFec[addr] = d.ldpcFec[addr] != d.bchBlock[i+j*ldpcBlockSize]
			}
		}
	}

	for i := 1; i < len(d.ldpcFec); i++ {
		d.ldpcFec[i] = d.ldpcFec[i] != d.ldpcFec[i-1]
	}
}

func (d *dvb2s) bitInterleave() {

}

func (d *dvb2s) mapIntoConstellation() {

	for i, j := 0, 0; i < len(d.fecFrame); i, j = i+d.bitsPerPlSymbol, j+1 {

		position := 0
		if d.fecFrame[i] {
			position |= 2
		}
		if d.fecFrame[i+1] {
			position |= 1
		}

		switch position {
		case 0x00:
			d.plFrame[j] = complex(map1Pi4, map1Pi4)
		case 0x01:
			d.plFrame[j] = complex(map1Pi4, -map1Pi4)
		case 0x02:
			d.plFrame[j] = complex(-map1Pi4, map1Pi4)
		case 0x03:
			d.plFrame[j] = complex(-map1Pi4, -map1Pi4)
		default:
			panic("unkwown symbol in mapper\n")
		}
	}
}

func (d *dvb2s) plHeaderEncode() {
	var plHeaderInt int
	isDvbs2x := (d.modcod & 0x80) > 0

	if isDvbs2x {
		plHeaderInt = d.modcod | (d.fecFrameType & 0x01)
	} else {
		plHeaderInt = (d.modcod << 2) | (d.fecFrameType & 0x03)
	}

	x := 0
	for i, value := range plHeaderGTable {
		if (plHeaderInt & (1 << uint(len(plHeaderGTable)-i))) > 0 {
			x ^= value
		}
	}

	plHeader := make([]bool, slotSize)
	copy(plHeader, plHeaderSof)
	plHeaderScrambled := plHeader[len(plHeaderSof):]
	plHeaderBit := (plHeaderInt & 0x01) > 0
	m := 1 << uint(len(plHeaderScrambled)/2-1)
	for i := 0; i < len(plHeaderScrambled); i += 2 {
		plHeaderScrambled[i] = (x & m) > 0
		plHeaderScrambled[i+1] = plHeaderScrambled[i] != plHeaderBit
		plHeaderScrambled[i] = plHeaderScrambled[i] != plHeaderScrambleTable[i]
		plHeaderScrambled[i+1] = plHeaderScrambled[i+1] != plHeaderScrambleTable[i+1]
		m >>= 1
	}

	for i := 0; i < len(d.plHeader); i += 2 {
		if plHeader[i] {
			d.plHeader[i] = complex(-map1Pi4, -map1Pi4)
		} else {
			d.plHeader[i] = complex(map1Pi4, map1Pi4)
		}
		if plHeader[i+1] {
			d.plHeader[i+1] = complex(map1Pi4, -map1Pi4)
		} else {
			d.plHeader[i+1] = complex(-map1Pi4, map1Pi4)
		}
	}
}

func (d *dvb2s) plScramble() {
	initX := 0x00001
	initY := 0x3ffff

	srx := initX
	sry := initY

	for i := range d.plFrame {
		fbx := (srx >> 0) ^ (srx >> 7)
		fby := (sry >> 0) ^ (sry >> 5) ^ (sry >> 7) ^ (sry >> 10)

		zx := (srx >> 4) ^ (srx >> 6) ^ (srx >> 15)
		zy := (sry >> 5) ^ (sry >> 6) ^ (sry >> 8) ^ (sry >> 9) ^
			(sry >> 10) ^ (sry >> 11) ^ (sry >> 12) ^ (sry >> 13) ^
			(sry >> 14) ^ (sry >> 15)

		r := (((srx ^ sry) & 1) | ((zx ^ zy) << 1)) & 0x03

		srx = ((srx >> 1) & 0x1ffff) | (fbx << 17)
		sry = ((sry >> 1) & 0x1ffff) | (fby << 17)

		switch r {
		case 0x00:
		case 0x01:
			d.plFrame[i] = complex(-imag(d.plFrame[i]), real(d.plFrame[i]))
		case 0x02:
			d.plFrame[i] = -d.plFrame[i]
		case 0x03:
			d.plFrame[i] = complex(imag(d.plFrame[i]), -real(d.plFrame[i]))
		default:
			panic("unkwown symbol in scrambler\n")
		}
	}
}

func (d *dvb2s) outInterpolateBbShape() { // TODO: preload and push forward the filter
	scale := 1.0 / float64(d.oversampling)
	j := 0
	if d.interpolateByRepeat {
		for _, value := range d.plFrame {
			value = complex(real(value)*scale, imag(value)*scale)
			for i := 0; i < d.oversampling; i++ {
				d.outFrame[j] = d.firFilter.fir(value)
				j++
			}
		}
	} else {
		nullValue := complex(0.0, 0.0)
		for _, value := range d.plFrame {
			d.outFrame[j] = d.firFilter.fir(value)
			j++
			for i := 1; i < d.oversampling; i++ {
				d.outFrame[j] = d.firFilter.fir(nullValue)
				j++
			}
		}
	}
}
