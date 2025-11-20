// rlwe_sample_export.go
package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"
)

func main() {
	logN := 4
	N := 1 << logN
	q := uint64(1 << 30)

	// ringQ, _ := ring.NewRing(N, []uint64{q})

	// Secret key s
	s := make([]int64, N)
	s[0] = 1
	s[2] = 1
	s[4] = 1

	// RLWE sample: (a, b = a*s + e)
	rand.Seed(time.Now().UnixNano())

	a := make([]int64, N)
	e := make([]int64, N)
	b := make([]int64, N)

	for i := 0; i < N; i++ {
		a[i] = rand.Int63n(int64(q))
		e[i] = rand.Int63n(5) - 2 // 작은 오류 [-2,2]
	}

	// b = a * s + e
	for i := 0; i < N; i++ {
		acc := int64(0)
		for j := 0; j < N; j++ {
			idx := (i - j + N) % N
			acc += a[j] * s[idx]
		}
		b[i] = (acc + e[i]) % int64(q)
		if b[i] < 0 {
			b[i] += int64(q)
		}
	}

	// export (a, b, q)
	data := map[string]interface{}{
		"a": a,
		"b": b,
		"q": q,
		"N": N,
	}

	file, _ := os.Create("rlwe_sample.json")
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encoder.Encode(data)

	fmt.Println("RLWE 샘플이 rlwe_sample.json 파일로 저장됨.")
}
