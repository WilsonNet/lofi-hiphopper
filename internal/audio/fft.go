package audio

import "math"

// fftInPlace is an iterative radix-2 Cooley-Tukey FFT. len(re) must be a
// power of two and len(re) == len(im).
func fftInPlace(re, im []float64) {
	n := len(re)
	if n <= 1 || len(im) != n || n&(n-1) != 0 {
		return
	}
	// Bit-reversal permutation.
	j := 0
	for i := 1; i < n; i++ {
		bit := n >> 1
		for j&bit != 0 {
			j ^= bit
			bit >>= 1
		}
		j ^= bit
		if i < j {
			re[i], re[j] = re[j], re[i]
			im[i], im[j] = im[j], im[i]
		}
	}

	for length := 2; length <= n; length <<= 1 {
		angle := -2 * math.Pi / float64(length)
		wLengthRe := math.Cos(angle)
		wLengthIm := math.Sin(angle)
		half := length >> 1
		for start := 0; start < n; start += length {
			wRe, wIm := 1.0, 0.0
			for offset := 0; offset < half; offset++ {
				left := start + offset
				right := left + half
				vRe := re[right]*wRe - im[right]*wIm
				vIm := re[right]*wIm + im[right]*wRe
				re[right] = re[left] - vRe
				im[right] = im[left] - vIm
				re[left] += vRe
				im[left] += vIm
				wRe, wIm = wRe*wLengthRe-wIm*wLengthIm, wRe*wLengthIm+wIm*wLengthRe
			}
		}
	}
}

func nextPowerOfTwo(n int) int {
	if n <= 1 {
		return 1
	}
	power := 1
	for power < n {
		power <<= 1
	}
	return power
}
