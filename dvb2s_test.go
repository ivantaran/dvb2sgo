package dvb2s

import (
	"bufio"
	"fmt"
	"math/cmplx"
	"os"
	"strconv"
	"strings"
	"testing"
)

const floatTolerance float64 = 1.0e-10

var bchInitVector = [...]bool{
	true, false, true, false, false, true, true, true,
	false, false, false, true, false, false, true, true,
	false, false, false, false, false, true, true, true,
	false, true, false, false, false, false, false, true,
	true, true, false, false, false, false, true, false,
	false, false, true, false, true, true, true, false,
	false, false, true, false, true, false, false, false,
	true, false, false, false, true, true, true, false,
	false, false, true, false, true, false, false, false,
	false, true, true, false, false, true, true, true,
	true, false, false, true, false, true, true, false,
	false, true, true, false, true, true, false, false,
	false, true, true, false, true, true, true, false,
	false, false, false, true, true, false, true, false,
	true, false, false, false, false, true, false, false,
	false, true, false, false, false, true, false, false,
	true, false, false, false, false, false, false, true,
	true, false, true, false, false, false, true, true,
	true, true, false, false, false, false, true, false,
	true, true, true, true, true, false, true, true,
	true, false, true, true, false, false, true, true,
	false, false, false, false, false, false, false, true,
	false, false, true, false, true, false, true, false,
	true, true, true, true, false, false, true, true,
	true,
}

func TestDvb2sCreating(t *testing.T) {
	t.Run("creating dvb2s object", func(t *testing.T) {
		if newDvb2s("normal", 2, true) == nil {
			t.Error("new dvb2s is nil")
		}
	})
}

func TestDvb2sLoad(t *testing.T) {
	t.Run("dvb2s loading", func(t *testing.T) {
		d := newDvb2s("normal", 2, true)
		err := d.LoadInputData("../../dvb_s2_qpsk_34/0_data.txt")
		if err != nil {
			t.Error(err)
		}
	})
}

func TestDvb2sBbFrameScramble(t *testing.T) {
	t.Run("Dvb2sBbFrameScramble", func(t *testing.T) {
		d := newDvb2s("normal", 2, true)
		d.LoadInputData("../../dvb_s2_qpsk_34/2_merger_slicer.txt")
		d.bbFrameScramble()

		file, err := os.Open("../../dvb_s2_qpsk_34/3_bbscrambler.txt")

		if err != nil {
			t.Error(err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)

		for i := 0; (i < len(d.bbFrame)) && scanner.Scan(); i++ {
			v, _ := strconv.Atoi(strings.TrimSpace(scanner.Text()))
			if (v == 1) != d.bbFrame[i] {
				t.Errorf("[%d]: %t - %t\n", i, v == 1, d.bbFrame[i])
				if i > 20 {
					break
				}
			}
		}
	})
}

func TestDvb2sInitBch(t *testing.T) {
	t.Run("dvb2s init BCH", func(t *testing.T) {
		d := newDvb2s("normal", 2, true)
		b := make([]bool, 200) // TODO: 200 is magic number
		len := d.bchInit(b)
		if len != 193 {
			t.Errorf("len != 193: len = %d\n", len)
		}
		for i := range bchInitVector {
			if b[i] != bchInitVector[i] {
				t.Errorf("error at position: %d\n", i)
			}
		}
	})
}

func TestDvb2sBchEncode(t *testing.T) {
	t.Run("dvb2s encode BCH", func(t *testing.T) {
		d := newDvb2s("normal", 2, true)
		d.LoadInputData("../../dvb_s2_qpsk_34/2_merger_slicer.txt")
		d.bbFrameScramble()
		gpoly := make([]bool, 200) // TODO: 200 is magic number
		d.bchInit(gpoly)
		d.bchEncode(gpoly)

		file, err := os.Open("../../dvb_s2_qpsk_34/4_bchencoder.txt")

		if err != nil {
			t.Error(err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)

		var i int
		j := 1
		for scanner.Scan() {
			if i > len(d.bbFrame) {
				v, _ := strconv.Atoi(strings.TrimSpace(scanner.Text()))
				if (v == 1) != d.bchFec[j] {
					t.Errorf("%t : %t\n", v == 1, d.bchFec[j])
				}
				j++
				if j == len(d.bchFec) {
					fmt.Printf("BCH len == 192: position = %d\n", i+1)
					break
				}
			}
			i++
		}
	})
}

func TestDvb2sLdpcEncode(t *testing.T) {
	t.Run("dvb2s encode LDPC", func(t *testing.T) {
		d := newDvb2s("normal", 2, true)
		d.LoadInputData("../../dvb_s2_qpsk_34/2_merger_slicer.txt")
		d.bbFrameScramble()
		gpoly := make([]bool, 200) // TODO: 200 is magic number

		d.bchInit(gpoly)
		d.bchEncode(gpoly)
		d.ldpcEncode()

		file, err := os.Open("../../dvb_s2_qpsk_34/5_ldpcencoder.txt")

		if err != nil {
			t.Error(err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)

		buffer := make([]bool, len(d.fecFrame))
		for i := 0; i < len(d.fecFrame) && scanner.Scan(); i++ {
			v, _ := strconv.Atoi(strings.TrimSpace(scanner.Text()))
			buffer[i] = v == 1
		}

		ldpcFec := buffer[len(d.bchBlock):]
		for i := range ldpcFec {
			if ldpcFec[i] != d.ldpcFec[i] {
				t.Errorf("[%d] %t : %t\n", i, ldpcFec[i], d.ldpcFec[i])
			}
		}
	})
}

func TestDvb2sMapIntoConstellation(t *testing.T) {
	t.Run("Dvb2sMapIntoConstellation", func(t *testing.T) {
		d := newDvb2s("normal", 2, true)
		d.LoadInputData("../../dvb_s2_qpsk_34/2_merger_slicer.txt")
		d.bbFrameScramble()
		gpoly := make([]bool, 200) // TODO: 200 is magic number

		d.bchInit(gpoly)
		d.bchEncode(gpoly)
		d.ldpcEncode()
		d.mapIntoConstellation()

		file, err := os.Open("../../dvb_s2_qpsk_34/7_mapper.txt")

		if err != nil {
			t.Error(err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)

		for i := 0; i < len(d.plFrame) && scanner.Scan(); i++ {
			s := strings.TrimSpace(scanner.Text())
			s = strings.Replace(s, "- ", "-", -1)
			s = strings.Replace(s, "+ ", "+", -1)
			s = strings.TrimSuffix(s, "j")

			list := strings.Split(s, " ")
			if len(list) != 2 {
				break
			}

			real, _ := strconv.ParseFloat(list[0], 64)
			imag, _ := strconv.ParseFloat(list[1], 64)
			c := complex(real, imag)
			if cmplx.Abs(d.plFrame[i]-c) > floatTolerance {
				t.Errorf("[%d]: %f != %f\n", i, d.plFrame[i], c)
			}
		}
	})
}

func TestDvb2sPlHeaderEncode(t *testing.T) {
	t.Run("Dvb2sPlHeaderEncode", func(t *testing.T) {
		d := newDvb2s("normal", 2, true)
		d.LoadInputData("../../dvb_s2_qpsk_34/2_merger_slicer.txt")
		d.bbFrameScramble()
		gpoly := make([]bool, 200) // TODO: 200 is magic number

		d.bchInit(gpoly)
		d.bchEncode(gpoly)
		d.ldpcEncode()
		d.mapIntoConstellation()
		d.plHeaderEncode()

		file, err := os.Open("../../dvb_s2_qpsk_34/8_plframer.txt")
		// file, err := os.Open("../../dvb_s2_qpsk_34/9_plscrambler.txt")

		if err != nil {
			t.Error(err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)

		for i := 0; i < len(d.plHeader) && scanner.Scan(); i++ {
			s := strings.TrimSpace(scanner.Text())
			s = strings.Replace(s, "- ", "-", -1)
			s = strings.Replace(s, "+ ", "+", -1)
			s = strings.TrimSuffix(s, "j")

			list := strings.Split(s, " ")
			if len(list) != 2 {
				break
			}

			real, _ := strconv.ParseFloat(list[0], 64)
			imag, _ := strconv.ParseFloat(list[1], 64)
			c := complex(real, imag)
			if cmplx.Abs(d.plHeader[i]-c) > floatTolerance {
				t.Errorf("[%d]: %f != %f with range %f\n", i, d.plHeader[i], c, cmplx.Abs(d.plHeader[i]-c))
			}
		}
	})
}

func TestDvb2sPlFrameScramble(t *testing.T) {
	t.Run("Dvb2sPlFrameScramble", func(t *testing.T) {
		d := newDvb2s("normal", 2, true)
		d.LoadInputData("../../dvb_s2_qpsk_34/2_merger_slicer.txt")
		d.bbFrameScramble()
		gpoly := make([]bool, 200) // TODO: 200 is magic number

		d.bchInit(gpoly)
		d.bchEncode(gpoly)
		d.ldpcEncode()
		d.mapIntoConstellation()
		d.plHeaderEncode()
		d.plScramble()

		file, err := os.Open("../../dvb_s2_qpsk_34/9_plscrambler.txt")

		if err != nil {
			t.Error(err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)

		for i := 0; i < len(d.plHeader) && scanner.Scan(); i++ {

		}

		for i := 0; i < len(d.plFrame) && scanner.Scan(); i++ {
			s := strings.TrimSpace(scanner.Text())
			s = strings.Replace(s, "- ", "-", -1)
			s = strings.Replace(s, "+ ", "+", -1)
			s = strings.TrimSuffix(s, "j")

			list := strings.Split(s, " ")
			if len(list) != 2 {
				break
			}

			real, _ := strconv.ParseFloat(list[0], 64)
			imag, _ := strconv.ParseFloat(list[1], 64)
			c := complex(real, imag)
			if cmplx.Abs(d.plFrame[i]-c) > floatTolerance {
				t.Errorf("[%d]: %f != %f with range %f\n", i, d.plFrame[i], c, cmplx.Abs(d.plFrame[i]-c))
			}
		}
	})
}

func TestDvb2sOutInterpolateBbShape(t *testing.T) {
	t.Run("Dvb2sOutInterpolateBbShape", func(t *testing.T) {
		d := newDvb2s("normal", 2, false)
		d.LoadInputData("../../dvb_s2_qpsk_34/2_merger_slicer.txt")
		d.bbFrameScramble()
		gpoly := make([]bool, 200) // TODO: 200 is magic number

		d.bchInit(gpoly)
		d.bchEncode(gpoly)
		d.ldpcEncode()
		d.mapIntoConstellation()
		d.plHeaderEncode()
		d.plScramble()
		d.outInterpolateBbShape()

		file, err := os.Create("output.txt")

		if err != nil {
			t.Error(err)
		}
		defer file.Close()

		writer := bufio.NewWriter(file)
		for _, value := range d.outFrame {
			writer.WriteString(fmt.Sprintf("%f\t%f\n", real(value), imag(value)))
		}

		writer.Flush()
	})
}

func TestDvb2sBbHeader(t *testing.T) {
	t.Run("TestDvb2sBbHeader", func(t *testing.T) {
		h := newBbHeader()
		if h == nil {
			t.Error("bbheader is nil")
		}
	})
}
