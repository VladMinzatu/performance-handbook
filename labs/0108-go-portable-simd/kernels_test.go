//go:build goexperiment.simd

package simdlab

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"
)

func data(n int) (x, y []float32) {
	x, y = make([]float32, n), make([]float32, n)
	for i := range x {
		x[i] = 2*rand.Float32() - 1
		y[i] = 2*rand.Float32() - 1
	}
	return
}

func TestDotAgree(t *testing.T) {
	for _, n := range []int{0, 1, 3, 4, 7, 16, 17, 63, 1000, 4099} {
		x, y := data(n)
		want := float64(DotScalar(x, y))
		for name, f := range map[string]func(x, y []float32) float32{
			"scalar4": DotScalar4, "simd1": DotSIMD1, "simd4": DotSIMD4,
		} {
			if got := float64(f(x, y)); math.Abs(got-want) > 1e-3*(1+math.Abs(want)) {
				t.Errorf("n=%d %s: got %v want %v", n, name, got, want)
			}
		}
	}
}

func TestAddAgree(t *testing.T) {
	for _, n := range []int{0, 1, 5, 16, 33, 1000} {
		x, y := data(n)
		a, b := make([]float32, n), make([]float32, n)
		AddScalar(a, x, y)
		AddSIMD(b, x, y)
		for i := range a {
			if a[i] != b[i] {
				t.Fatalf("n=%d i=%d: %v != %v", n, i, a[i], b[i])
			}
		}
	}
}

// Sizes chosen to land in L1 (4K floats = 16KB x2), L2, and DRAM.
var sizes = []int{1 << 10, 1 << 12, 1 << 16, 1 << 20, 1 << 24}

func benchDot(b *testing.B, f func(x, y []float32) float32) {
	for _, n := range sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			x, y := data(n)
			b.SetBytes(int64(n) * 8)
			var sink float32
			for b.Loop() {
				sink += f(x, y)
			}
			_ = sink
		})
	}
}

func BenchmarkDotScalar(b *testing.B)  { benchDot(b, DotScalar) }
func BenchmarkDotScalar4(b *testing.B) { benchDot(b, DotScalar4) }
func BenchmarkDotSIMD1(b *testing.B)   { benchDot(b, DotSIMD1) }
func BenchmarkDotSIMD4(b *testing.B)   { benchDot(b, DotSIMD4) }

func benchAdd(b *testing.B, f func(dst, x, y []float32)) {
	for _, n := range sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			x, y := data(n)
			dst := make([]float32, n)
			b.SetBytes(int64(n) * 12)
			for b.Loop() {
				f(dst, x, y)
			}
		})
	}
}

func BenchmarkAddScalar(b *testing.B) { benchAdd(b, AddScalar) }
func BenchmarkAddSIMD(b *testing.B)   { benchAdd(b, AddSIMD) }
