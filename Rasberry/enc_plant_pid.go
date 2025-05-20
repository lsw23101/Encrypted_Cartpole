package main

import (
	"Enc_control_RLWE/com_utils"
	"bufio"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
	"time"

	"go.bug.st/serial"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	_ "github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/ring"
	_ "github.com/tuneinsight/lattigo/v6/ring"
	"github.com/tuneinsight/lattigo/v6/schemes/bgv"
	_ "github.com/tuneinsight/lattigo/v6/schemes/bgv"
)

func main() {
	// 시리얼 포트 설정
	mode := &serial.Mode{
		BaudRate: 115200,
	}
	//port, err := serial.Open("/dev/ttyACM0", mode) // 아두이노와 라즈베리파이 연결 포트
	port, err := serial.Open("COM4", mode) // 컴퓨터랑 연결할때 COM4에 연결
	if err != nil {
		fmt.Println("아두이노와 실패:", err)
		return
	}
	defer port.Close()
	fmt.Println("포트 연결됨. 아두이노 리셋 대기 중...")
	time.Sleep(1 * time.Second)

	// TCP 클라이언트 연결
	//conn, err := net.Dial("tcp", "192.168.0.5:8080")
	conn, err := net.Dial("tcp", "127.0.0.1:8080")
	if err != nil {
		fmt.Println("TCP 연결 실패:", err)
		return
	}
	defer conn.Close()
	fmt.Println("컨트롤러 서버와 TCP 연결됨")

	serialReader := bufio.NewReader(port)

	////// 여기서부터 암호화 하기 ///////

	// log2 of polynomial degree
	logN := 12
	// Choose the size of plaintext modulus (2^ptSize)
	ptSize := uint64(28)
	// Choose the size of ciphertext modulus (2^ctSize)
	ctSize := int(74)

	r := 0.0010

	// ============== Encryption settings ==============
	// Search a proper prime to set plaintext modulus
	primeGen := ring.NewNTTFriendlyPrimesGenerator(ptSize, uint64(math.Pow(2, float64(logN)+1)))
	ptModulus, _ := primeGen.NextAlternatingPrime()
	fmt.Println("Plaintext modulus:", ptModulus)

	// Create a chain of ciphertext modulus
	logQ := []int{int(math.Floor(float64(ctSize) * 0.5)), int(math.Ceil(float64(ctSize) * 0.5))}

	// Parameters satisfying 128-bit security
	// BGV scheme is used
	params, _ := bgv.NewParametersFromLiteral(bgv.ParametersLiteral{
		LogN:             logN,
		LogQ:             logQ,
		PlaintextModulus: ptModulus,
	})
	fmt.Println("Ciphertext modulus:", params.QBigInt())
	fmt.Println("Degree of polynomials:", params.N())

	//	Generate secret key
	kgen := bgv.NewKeyGenerator(params)
	sk := kgen.GenSecretKeyNew()

	// 암호화, 패킹에 필요한 객체
	encryptor := bgv.NewEncryptor(params, sk)
	decryptor := bgv.NewDecryptor(params, sk)
	encoder := bgv.NewEncoder(params)

	pt_angle := bgv.NewPlaintext(params, params.MaxLevel())

	for {
		loopStart := time.Now()

		// ① 시리얼 읽기
		t1 := time.Now()
		line, err := serialReader.ReadString('\n')
		t2 := time.Now()
		if err != nil {
			fmt.Println("읽기 오류:", err)
			continue
		}
		fmt.Println(">>", line)

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 콤마가 포함되어 있으면 무시 (예전 형식 대비)
		if strings.Contains(line, ",") {
			fmt.Println("무시됨 (콤마 포함):", line)
			continue
		}

		// angle 값만 파싱
		angle, err := strconv.ParseFloat(line, 64)
		if err != nil {
			fmt.Println("Angle 파싱 실패:", err)
			continue
		}

		fmt.Printf("① 시리얼 수신 완료 (%v)\n", t2.Sub(t1))
		fmt.Println("받은 데이터 (Angle):", angle)

		// 여기서 암호화
		quantized_angle := math.Round((1 / r) * angle)
		fmt.Println("양자화 된 각도:", quantized_angle)

		encode := make([]int64, 1)
		encode[0] = int64(quantized_angle)

		encoder.Encode(encode, pt_angle)
		// fmt.Println("pt_angle 된 각도 :", pt_angle)
		ct_angle, _ := encryptor.EncryptNew(pt_angle)

		// fmt.Printf("암호화 된 각도\n", ct_angle)

		// 사이즈 확인하기
		serialized_angle, err := ct_angle.MarshalBinary() // 이런 식으로

		fmt.Println("Y 크기.", len(serialized_angle)) // >> 131406

		// fmt.Println("Y serialized_angle.", serialized_angle[len(serialized_angle)-20:]) // >> 131406
		// ② TCP 송신
		t3 := time.Now()
		_, err = conn.Write([]byte(serialized_angle)) // 리스트 값을 문자열로 전송
		if err != nil {
			fmt.Println("출력값 전송 실패:", err)
			break
		}
		t4 := time.Now()
		fmt.Printf("② TCP 전송 완료 (%v)\n", t4.Sub(t3))

		// _, err = conn.Write([]byte(line + "\n"))
		// if err != nil {
		// 	fmt.Println("서버로 데이터 전송 실패:", err)
		// 	break
		// }

		// ③ TCP 응답 수신
		t5 := time.Now()
		response, err := com_utils.ReadFullData(conn, 131406)
		t6 := time.Now()
		if err != nil {
			fmt.Println("Uout 데이터 수신 실패:", err)
			return
		}
		fmt.Printf("③ TCP 응답 수신 완료 (%v)\n", t6.Sub(t5))

		// fmt.Println("response .", response[len(serialized_angle)-20:]) // >> 131406

		// 여기서 제어 입력 받는 부분 데이터처리리
		ct_U := rlwe.NewCiphertext(params, params.MaxLevel())
		err = ct_U.UnmarshalBinary(response[:131406])
		if err != nil {
			fmt.Println("Ciphertext 역직렬화 실패:", err)
			return
		}

		// fmt.Println("ct_U 받은거 확인:", ct_U)

		dec_U := decryptor.DecryptNew(ct_U)
		// dec_angle := decryptor.DecryptNew(ct_angle)

		// 디코딩된 값 저장할 슬라이스 준비
		decoded := make([]int64, 1) // slots는 사용하는 슬롯 수 (예: 10)
		// decoded_angle := make([]int64, 1)
		// 디코딩 실행 (in-place)
		encoder.Decode(dec_U, decoded)
		// encoder.Decode(dec_angle, decoded_angle)

		// fmt.Println("dec_U 계산 결과:", dec_U)

		// 제어 입력의 첫 번째 값 사용
		// pwm_u := int16(decoded[0])

		fmt.Println("PID 계산 결과:", decoded)

		pwm_u := float64(decoded[0]) * r // 여기서 decoded는 int지만 pwm_u는 float가 되어야함

		// PWM 값 송신
		t7 := time.Now()
		_, err = port.Write([]byte(fmt.Sprintf("%d,%d\n", int(pwm_u), 1))) // PWM과 방향을 시리얼로 전송
		t8 := time.Now()
		if err != nil {
			fmt.Println("아두이노로 전송 실패:", err)
			break
		}
		fmt.Printf("④ 시리얼 송신 완료 (%v)\n", t8.Sub(t7))

		time.Sleep(1 * time.Millisecond)

		loopElapsed := time.Since(loopStart)
		fmt.Printf("총 루프 처리 시간: %v\n", loopElapsed)
		fmt.Println("-----------------------------------")
	}
}
