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
	"github.com/tuneinsight/lattigo/v6/ring"
	"github.com/tuneinsight/lattigo/v6/schemes/bgv"
)

func main() {
	// 시리얼 포트 설정
	mode := &serial.Mode{
		BaudRate: 115200,
	}

	port, err := serial.Open("/dev/ttyACM0", mode)
	if err != nil {
		fmt.Println("아두이노와 실패:", err)
		return
	}

	fmt.Println("포트 연결됨. 아두이노 리셋 대기 중...")
	time.Sleep(1 * time.Second)

	// TCP 클라이언트 연결
	conn, err := net.Dial("tcp", "192.168.0.5:8080")
	if err != nil {
		fmt.Println("TCP 연결 실패:", err)
		resetAndClosePort(port)
		return
	}
	defer conn.Close()
	fmt.Println("컨트롤러 서버와 TCP 연결됨")

	serialReader := bufio.NewReader(port)

	////// 암호화 관련 설정 ///////

	logN := 12
	ptSize := uint64(28)
	ctSize := int(74)
	r := 0.0010

	primeGen := ring.NewNTTFriendlyPrimesGenerator(ptSize, uint64(math.Pow(2, float64(logN)+1)))
	ptModulus, _ := primeGen.NextAlternatingPrime()
	fmt.Println("Plaintext modulus:", ptModulus)

	logQ := []int{int(math.Floor(float64(ctSize) * 0.5)), int(math.Ceil(float64(ctSize) * 0.5))}

	params, _ := bgv.NewParametersFromLiteral(bgv.ParametersLiteral{
		LogN:             logN,
		LogQ:             logQ,
		PlaintextModulus: ptModulus,
	})
	fmt.Println("Ciphertext modulus:", params.QBigInt())
	fmt.Println("Degree of polynomials:", params.N())

	kgen := bgv.NewKeyGenerator(params)
	sk := kgen.GenSecretKeyNew()

	encryptor := bgv.NewEncryptor(params, sk)
	decryptor := bgv.NewDecryptor(params, sk)
	encoder := bgv.NewEncoder(params)

	pt_angle := bgv.NewPlaintext(params, params.MaxLevel())

	for {
		loopStart := time.Now()

		line, err := serialReader.ReadString('\n')
		if err != nil {
			fmt.Println("읽기 오류:", err)
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		lines := strings.Split(line, "\n")
		lastLine := strings.TrimSpace(lines[len(lines)-1])
		if lastLine == "" {
			continue
		}

		if strings.Contains(lastLine, ",") {
			fmt.Println("무시됨 (콤마 포함):", lastLine)
			continue
		}

		angle, err := strconv.ParseFloat(lastLine, 64)
		if err != nil {
			fmt.Println("Angle 파싱 실패:", err)
			continue
		}

		serial_start := time.Now()
		fmt.Printf("1. 시리얼 통신 시간: %v\n", time.Since(serial_start))
		fmt.Println("받은 데이터 (Angle):", angle)

		quantized_angle := math.Round((1 / r) * angle)
		encode := make([]int64, 1)
		encode[0] = int64(quantized_angle)

		encoder.Encode(encode, pt_angle)
		ct_angle, _ := encryptor.EncryptNew(pt_angle)

		enc_start := time.Now()
		serialized_angle, err := ct_angle.MarshalBinary()
		if err != nil {
			fmt.Println("암호문 직렬화 실패:", err)
			continue
		}
		fmt.Printf("\n2. 암호화 및 데이터 변환 시간: %v\n", time.Since(enc_start))

		com_start := time.Now()
		_, err = conn.Write(serialized_angle)
		if err != nil {
			fmt.Println("출력값 전송 실패:", err)
			break
		}

		response, err := com_utils.ReadFullData(conn, 131406)
		if err != nil {
			fmt.Println("Uout 데이터 수신 실패:", err)
			break
		}
		fmt.Printf("3. 암호 데이터의 TCP 송수신 시간: %v\n", time.Since(com_start))

		dec_start := time.Now()
		ct_U := rlwe.NewCiphertext(params, params.MaxLevel())
		err = ct_U.UnmarshalBinary(response[:131406])
		if err != nil {
			fmt.Println("Ciphertext 역직렬화 실패:", err)
			break
		}

		dec_U := decryptor.DecryptNew(ct_U)

		decoded := make([]int64, 1)
		encoder.Decode(dec_U, decoded)

		pwm_u := float64(decoded[0]) * r
		fmt.Printf("4. 복호화 및 데이터 변환 시간: %v\n\n", time.Since(dec_start))
		fmt.Println("제어기가 수행하는 연산 Kp * e = 20.0 * 각도")
		fmt.Println("받은 제어 결과 :", pwm_u)

		serial_2__start := time.Now()
		_, err = port.Write([]byte(fmt.Sprintf("%d,%d\n", int(pwm_u), 1)))
		if err != nil {
			fmt.Println("아두이노로 전송 실패:", err)
			break
		}
		fmt.Printf("\n5. 시리얼 송신 시간: %v\n", time.Since(serial_2__start))

		loopElapsed := time.Since(loopStart)
		fmt.Printf("총 루프 처리 시간: %v\n", loopElapsed)
		fmt.Println("-----------------------------------")
	}

	resetAndClosePort(port)
}

func resetAndClosePort(port serial.Port) {
	fmt.Println("포트 버퍼 초기화 및 닫기 시작...")
	if err := port.ResetInputBuffer(); err != nil {
		fmt.Println("입력 버퍼 초기화 실패:", err)
	}
	if err := port.ResetOutputBuffer(); err != nil {
		fmt.Println("출력 버퍼 초기화 실패:", err)
	}
	err := port.Close()
	if err != nil {
		fmt.Println("포트 닫기 실패:", err)
	} else {
		fmt.Println("포트 닫음. 2초 대기...")
		time.Sleep(2 * time.Second)
	}
	fmt.Println("포트 정리 완료.")
}
