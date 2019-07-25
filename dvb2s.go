package dvb2s

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type dvb2s struct {
	modcod          int
	fecFrameType    int
	oversampling    int
	ldpcQFactor     int
	bitsPerPlSymbol int
	bbFrame         []bool
	bchBlock        []bool
	bchFec          []bool
	ldpcFec         []bool
	fecFrame        []bool
	plFrame         []complex128
	plHeader        []complex128
}

func newDvb2s(fecFrameType string, oversampling int) *dvb2s {
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

	d.fecFrame = make([]bool, fecFrameSize)
	d.bbFrame = d.fecFrame[:bbFrameSize]
	d.bchBlock = d.fecFrame[:bchBlockSize]
	d.bchFec = d.fecFrame[bbFrameSize:bchBlockSize]
	d.ldpcFec = d.fecFrame[bchBlockSize:]
	d.plFrame = make([]complex128, plFrameSize)
	d.plHeader = make([]complex128, slotSize)

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
