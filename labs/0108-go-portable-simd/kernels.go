package simdlab

// Scalar reference kernels. These build on every GOEXPERIMENT setting.

// DotScalar is the naive loop: one accumulator, so every add depends on the
// previous one (a serial floating-point dependency chain).
func DotScalar(x, y []float32) float32 {
	var s float32
	for i := range x {
		s += x[i] * y[i]
	}
	return s
}

// DotScalar4 breaks the dependency chain with four independent accumulators.
// Its result differs slightly from DotScalar (float addition is not
// associative), which is the same reason a compiler can't do this for us.
func DotScalar4(x, y []float32) float32 {
	var s0, s1, s2, s3 float32
	n := len(x) &^ 3
	for i := 0; i < n; i += 4 {
		s0 += x[i] * y[i]
		s1 += x[i+1] * y[i+1]
		s2 += x[i+2] * y[i+2]
		s3 += x[i+3] * y[i+3]
	}
	for i := n; i < len(x); i++ {
		s0 += x[i] * y[i]
	}
	return (s0 + s1) + (s2 + s3)
}

// AddScalar computes dst[i] = x[i] + y[i].
func AddScalar(dst, x, y []float32) {
	for i := range dst {
		dst[i] = x[i] + y[i]
	}
}
