package main

import (
	"fmt"
	"math"
	"net"
	"time" // time 패키지 추가

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/ring"
	"github.com/tuneinsight/lattigo/v6/schemes/bgv"
)

func main() {
	// 컨트롤러 소켓 설정
	conn, err := net.Dial("tcp", "172.20.61.165:8080") // 라즈베리파이의 IP 주소와 포트
	if err != nil {
		fmt.Println("서버에 연결 실패:", err)
		return
	}
	defer conn.Close()

	// Generate plaintext modulus // 암호데이터 바이너리값

	logN := 12
	primeGen := ring.NewNTTFriendlyPrimesGenerator(18, uint64(math.Pow(2, float64(logN)+1)))
	prime, _ := primeGen.NextAlternatingPrime()

	// 128-bit secure BGV parameters
	params, _ := bgv.NewParametersFromLiteral(bgv.ParametersLiteral{
		LogN:             logN,
		LogQ:             []int{28, 28}, // 128bit security 맞추기 위해 Q가 줄어듦 QP에 맞춰야대서
		LogP:             []int{15},     // special modulus (P=우리가 아는 1/L)
		PlaintextModulus: prime,
	})

	ct0 := rlwe.NewCiphertext(params, params.MaxLevel())

	// 데이터 수신 버퍼 설정
	chunkSize := 1024

	buf := make([]byte, chunkSize) // 1024 바이트씩 수신
	// buf := make([]byte, 65000)

	// 데이터 수신을 위한 누적된 결과 저장
	var totalData []byte

	// 데이터 수신 시작 시간 기록
	startTime := time.Now()

	for {
		// 데이터 수신 (서버에서 전송한 바이너리 데이터 받기)
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Println("수신 오류:", err)
			break
		}

		// 수신된 데이터 누적
		totalData = append(totalData, buf[:n]...)

		// 만약 전체 데이터를 다 받았으면 종료
		if len(totalData) >= 131406 { // 예시로 131406 크기만큼 받으면 종료
			break
		}
	}

	// _, err = conn.Read(buf)
	// if err != nil {
	// 	fmt.Println("수신 오류:", err)
	// }

	err = ct0.UnmarshalBinary(totalData[:131406])
	if err != nil {
		// 오류 로그 출력
		fmt.Println("Ciphertext 역직렬화 실패:", err)
		return
	}
	fmt.Printf("전달받은 ct1의 사이즈:%v\n", ct0.BinarySize())

	// 데이터 수신 종료 시간 기록
	endTime := time.Now()

	// 통신 시간 계산
	communicationTime := endTime.Sub(startTime)

	// 결과 출력
	fmt.Printf("통신에 걸린 시간: %v\n", communicationTime)

	// // 다시 decode binary to Ploy

	// 복원된 Ciphertext 객체 출력
	// fmt.Printf("받은 ciphertext 객체: %+v\n", ct0)
}
