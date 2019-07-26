package dvb2s

type fir struct {
	taps         []complex128
	coefficients []float64
	offset       int
}

func newFir(coefficients []float64) *fir {
	var f fir

	f.coefficients = coefficients
	f.taps = make([]complex128, len(coefficients))
	f.offset = 0

	return &f
}

func (f *fir) fir(value complex128) complex128 {
	var r complex128

	f.taps[f.offset] = value

	for i := range f.taps {
		j := (i + f.offset) % len(f.taps)
		r += complex(f.coefficients[i]*real(f.taps[j]), f.coefficients[i]*imag(f.taps[j]))
	}

	if f.offset < len(f.taps)-1 {
		f.offset++
	} else {
		f.offset = 0
	}

	return r
}
