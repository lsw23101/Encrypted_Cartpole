package main

import (
	"fmt"
	"math"
	"net"
	"os"

	//"io"
	"math/rand"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/ring"
	"github.com/tuneinsight/lattigo/v6/schemes/bgv"
)

/*
https://github.com/tuneinsight/lattigo/blob/v6.1.0/ring/poly.go
마지막부분에 바이너리로 encode decode 하는 함수도 있음
*/

func main() {
	// 서버 소켓 설정

	listen, err := net.Listen("tcp", "172.20.61.165:8080") // 라즈베리파이 IP와 포트
	if err != nil {
		fmt.Println("서버 소켓 설정 실패:", err)
		os.Exit(1)
	}
	defer listen.Close()
	fmt.Println("플랜트 서버 실행 중...")

	// 클라이언트와 연결 수락
	conn, err := listen.Accept()
	if err != nil {
		fmt.Println("연결 수락 실패:", err)
		os.Exit(1)
	}
	defer conn.Close()
	fmt.Println("컨트롤러와 연결됨:", conn.RemoteAddr())

	//////  여기서부터 암호화   ///////

	slots := 10

	// Generate plaintext modulus
	logN := 12
	primeGen := ring.NewNTTFriendlyPrimesGenerator(18, uint64(math.Pow(2, float64(logN)+1)))
	prime, _ := primeGen.NextAlternatingPrime()
	fmt.Println("Plaintext modulus:", prime)

	// Plaintext messages
	m1 := make([]uint64, slots)

	for i := 0; i < slots; i++ {
		m1[i] = uint64(rand.Intn(1000))
	}

	fmt.Println("message 1:", m1)

	// 128-bit secure BGV parameters
	params, _ := bgv.NewParametersFromLiteral(bgv.ParametersLiteral{
		LogN:             logN,
		LogQ:             []int{28, 28}, // 128bit security 맞추기 위해 Q가 줄어듦
		LogP:             []int{15},     // special modulus
		PlaintextModulus: prime,
	})

	// fmt.Println("log2 plaintext modulus:", "around", math.Round(params.LogT()))
	// // level = number of possible multiplications (mostly)
	// fmt.Println("maximum level:", params.MaxLevel())
	// // actual ciphertext modulus = QP
	fmt.Println("ciphertext modulus:", params.QPBigInt(), "around 2^", math.Round(params.LogQP()))

	// Key Generator
	kgen := rlwe.NewKeyGenerator(params)

	// Secret Key
	sk := kgen.GenSecretKeyNew()

	// Encoder
	ecd := bgv.NewEncoder(params)

	// Encryptor
	enc := rlwe.NewEncryptor(params, sk)

	// Decryptor
	// dec := rlwe.NewDecryptor(params, sk)

	// Create empty plaintexts
	pt1 := bgv.NewPlaintext(params, params.MaxLevel())

	// NTT packing
	ecd.Encode(m1, pt1)

	// Encryption
	ct1, _ := enc.EncryptNew(pt1)
	// ct0 := rlwe.NewCiphertext(params, params.MaxLevel())

	fmt.Println("ct.BinarySize", ct1.BinarySize())
	//fmt.Println("zero ciphertext: %+v\n", ct0)
	// fmt.Println("ct1 값들 : %+v\n", ct1)
	// fmt.Println("ct1 값들 : %+v\n", ct1)
	// fmt.Println("ct1 Value[0] : %+v\n", ct1.Value[0])

	// ct1을 바이너리 직렬화
	bin_ct1, err := ct1.MarshalBinary()
	if err != nil {
		fmt.Println("바이너리 직렬화 실패:", err)
		return
	}
	// fmt.Printf("binary ct1: %+v\n", bin_ct1)
	fmt.Println("lenth of binary ct1: ", len(bin_ct1)) // ct1.BinarySize와 같음

	// // 청크 크기 설정 (예: 1024 바이트씩 나누어 전송)
	// chunkSize := 1024

	// // 데이터를 작은 청크로 나누어 전송
	// for i := 0; i < len(bin_ct1); i += chunkSize {
	// 	end := i + chunkSize
	// 	if end > len(bin_ct1) {
	// 		end = len(bin_ct1)
	// 	}
	// 	_, err := conn.Write(bin_ct1[i:end])
	// 	if err != nil {
	// 		fmt.Println("출력값 전송 실패:", err)
	// 		return
	// 	}
	// 	// fmt.Printf("전송된 데이터: %d bytes\n", n)
	// }
	_, err = conn.Write(bin_ct1)
	if err != nil {
		fmt.Println("출력값 전송 실패:", err)
		return
	}
	// 전송 완료 후
	fmt.Println("ct1 전송 완료.")

	// fmt.Printf("ct1: ",ct1)
	// fmt.Printf("sk:", sk)
}
