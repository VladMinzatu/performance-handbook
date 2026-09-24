//go:build goexperiment.simd

package simdlab

import "simd"

// hsum is the horizontal reduction. The portable API has no reduce
// operation, so spill the lanes and add them in scalar code.
func hsum(v simd.Float32s, buf []float32) float32 {
	v.Store(buf)
	var s float32
	for _, e := range buf[:v.Len()] {
		s += e
	}
	return s
}

// DotSIMD1 uses one vector accumulator: lane-parallel, but each FMA still
// depends on the previous one.
func DotSIMD1(x, y []float32) float32 {
	var acc simd.Float32s
	w := acc.Len()
	i := 0
	for ; i+w <= len(x); i += w {
		acc = simd.LoadFloat32s(x[i:]).MulAdd(simd.LoadFloat32s(y[i:]), acc)
	}
	var buf [64]float32
	s := hsum(acc, buf[:])
	for ; i < len(x); i++ {
		s += x[i] * y[i]
	}
	return s
}

// DotSIMD4 uses four independent vector accumulators.
func DotSIMD4(x, y []float32) float32 {
	var a0, a1, a2, a3 simd.Float32s
	w := a0.Len()
	i := 0
	for ; i+4*w <= len(x); i += 4 * w {
		a0 = simd.LoadFloat32s(x[i:]).MulAdd(simd.LoadFloat32s(y[i:]), a0)
		a1 = simd.LoadFloat32s(x[i+w:]).MulAdd(simd.LoadFloat32s(y[i+w:]), a1)
		a2 = simd.LoadFloat32s(x[i+2*w:]).MulAdd(simd.LoadFloat32s(y[i+2*w:]), a2)
		a3 = simd.LoadFloat32s(x[i+3*w:]).MulAdd(simd.LoadFloat32s(y[i+3*w:]), a3)
	}
	var buf [64]float32
	s := hsum(a0.Add(a1).Add(a2.Add(a3)), buf[:])
	for ; i < len(x); i++ {
		s += x[i] * y[i]
	}
	return s
}

// AddSIMD computes dst[i] = x[i] + y[i], a vector at a time.
func AddSIMD(dst, x, y []float32) {
	w := simd.BroadcastFloat32s(0).Len()
	i := 0
	for ; i+w <= len(dst); i += w {
		simd.LoadFloat32s(x[i:]).Add(simd.LoadFloat32s(y[i:])).Store(dst[i:])
	}
	for ; i < len(dst); i++ {
		dst[i] = x[i] + y[i]
	}
}
