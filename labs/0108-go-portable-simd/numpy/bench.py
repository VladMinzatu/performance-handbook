"""NumPy counterparts of the Go kernels, same sizes and same MB/s accounting."""
import timeit
import numpy as np

SIZES = [1 << 10, 1 << 12, 1 << 16, 1 << 20, 1 << 24]
rng = np.random.default_rng(0)


def bench(name, fn, n, bytes_per_elem):
    reps, best = 1, None
    while True:  # grow reps until a run takes >= 200ms
        t = timeit.timeit(fn, number=reps)
        if t >= 0.2:
            break
        reps *= 4
    best = min(timeit.repeat(fn, number=reps, repeat=5)) / reps
    print(f"{name}/n={n}\t{best * 1e9:12.0f} ns/op\t{n * bytes_per_elem / best / 1e6:9.1f} MB/s")


for n in SIZES:
    x = (2 * rng.random(n, dtype=np.float32) - 1)
    y = (2 * rng.random(n, dtype=np.float32) - 1)
    dst = np.empty(n, dtype=np.float32)
    bench("Dot(np.dot)", lambda: np.dot(x, y), n, 8)
    bench("Dot(x*y).sum", lambda: (x * y).sum(), n, 8)
    bench("Add(alloc)", lambda: x + y, n, 12)
    bench("Add(out=)", lambda: np.add(x, y, out=dst), n, 12)
