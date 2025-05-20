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

	port, err := serial.Open("COM4", mode) // 컴퓨터랑 연결할때 COM4에 연결
	if err != nil {
		fmt.Println("아두이노와 실패:", err)
		return
	}
	// defer port.Close() // 직접 닫을 예정

	fmt.Println("포트 연결됨. 아두이노 리셋 대기 중...")
	time.Sleep(1 * time.Second)

	// TCP 클라이언트 연결
	conn, err := net.Dial("tcp", "127.0.0.1:8080")
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

		// 루프 초반에 버퍼 비우기 (불필요한 이전 데이터 제거)
		if err := port.ResetInputBuffer(); err != nil {
			fmt.Println("입력 버퍼 초기화 실패:", err)
		}
		if err := port.ResetOutputBuffer(); err != nil {
			fmt.Println("출력 버퍼 초기화 실패:", err)
		}

		// ① 시리얼에서 읽기 (여러 줄 올 수 있으니 마지막 줄만 처리)
		line, err := serialReader.ReadString('\n')
		if err != nil {
			fmt.Println("읽기 오류:", err)
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 여러 줄이 들어왔다면 마지막 줄만 사용
		lines := strings.Split(line, "\n")
		lastLine := strings.TrimSpace(lines[len(lines)-1])
		if lastLine == "" {
			continue
		}

		// 콤마가 포함된 줄은 아두이노 각도 값이 아니므로 무시
		if strings.Contains(lastLine, ",") {
			fmt.Println("무시됨 (콤마 포함):", lastLine)
			continue
		}

		angle, err := strconv.ParseFloat(lastLine, 64)
		if err != nil {
			fmt.Println("Angle 파싱 실패:", err)
			continue
		}

		fmt.Printf("① 시리얼 수신 완료 (마지막 줄) (%v 누적)\n", time.Since(loopStart))
		fmt.Println("받은 데이터 (Angle):", angle)

		// 암호화 및 양자화
		quantized_angle := math.Round((1 / r) * angle)
		fmt.Println("양자화 된 각도:", quantized_angle)

		encode := make([]int64, 1)
		encode[0] = int64(quantized_angle)

		encoder.Encode(encode, pt_angle)
		ct_angle, _ := encryptor.EncryptNew(pt_angle)

		serialized_angle, err := ct_angle.MarshalBinary()
		if err != nil {
			fmt.Println("암호문 직렬화 실패:", err)
			continue
		}

		// fmt.Println("Y 크기:", len(serialized_angle))

		fmt.Printf("데이터 후처리 (%v 누적)\n", time.Since(loopStart))

		// ② TCP 송신
		_, err = conn.Write(serialized_angle)
		if err != nil {
			fmt.Println("출력값 전송 실패:", err)
			break
		}
		fmt.Printf("② TCP 전송 완료 (%v 누적)\n", time.Since(loopStart))

		// ③ TCP 응답 수신
		response, err := com_utils.ReadFullData(conn, 131406)
		if err != nil {
			fmt.Println("Uout 데이터 수신 실패:", err)
			break
		}
		fmt.Printf("③ TCP 응답 수신 완료 (%v 누적)\n", time.Since(loopStart))

		ct_U := rlwe.NewCiphertext(params, params.MaxLevel())
		err = ct_U.UnmarshalBinary(response[:131406])
		if err != nil {
			fmt.Println("Ciphertext 역직렬화 실패:", err)
			break
		}

		dec_U := decryptor.DecryptNew(ct_U)

		decoded := make([]int64, 1)
		encoder.Decode(dec_U, decoded)

		fmt.Println("PID 계산 결과:", decoded)

		pwm_u := float64(decoded[0]) * r

		fmt.Printf("데이터 후처리(%v 누적)\n", time.Since(loopStart))
		// PWM 값 송신
		_, err = port.Write([]byte(fmt.Sprintf("%d,%d\n", int(pwm_u), 1)))
		if err != nil {
			fmt.Println("아두이노로 전송 실패:", err)
			break
		}
		fmt.Printf("④ 시리얼 송신 완료 (%v 누적)\n", time.Since(loopStart))

		// 얘 시간이 아두이노가 보내주는 시간보다 훨씬 짧도록
		// time.Sleep(1 * time.Millisecond)

		loopElapsed := time.Since(loopStart)
		fmt.Printf("총 루프 처리 시간: %v\n", loopElapsed)
		fmt.Println("-----------------------------------")
	}

	// 루프 종료 시 포트 닫기 전에 버퍼 비우고 닫기
	resetAndClosePort(port)
}

// 포트 버퍼 비우고 닫고 2초 대기하는 함수
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
		time.Sleep(2 * time.Second) // OS가 포트 해제할 시간 확보
	}
	fmt.Println("포트 정리 완료.")
}
