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
	port, err := setupSerial()
	if err != nil {
		fmt.Println("시리얼 연결 실패:", err)
		return
	}
	defer resetAndClosePort(port)

	conn, err := net.Dial("tcp", "192.168.0.5:8080")
	if err != nil {
		fmt.Println("TCP 연결 실패:", err)
		return
	}
	defer conn.Close()
	fmt.Println("TCP 연결됨")

	logN := 12
	ptSize := uint64(28)
	ctSize := int(74)
	r := 0.0010

	primeGen := ring.NewNTTFriendlyPrimesGenerator(ptSize, uint64(math.Pow(2, float64(logN)+1)))
	ptModulus, _ := primeGen.NextAlternatingPrime()

	logQ := []int{int(math.Floor(float64(ctSize) * 0.5)), int(math.Ceil(float64(ctSize) * 0.5))}
	params, _ := bgv.NewParametersFromLiteral(bgv.ParametersLiteral{
		LogN:             logN,
		LogQ:             logQ,
		PlaintextModulus: ptModulus,
	})

	kgen := bgv.NewKeyGenerator(params)
	sk := kgen.GenSecretKeyNew()

	encryptor := bgv.NewEncryptor(params, sk)
	decryptor := bgv.NewDecryptor(params, sk)
	encoder := bgv.NewEncoder(params)
	pt_angle := bgv.NewPlaintext(params, params.MaxLevel())

	angleChan := make(chan float64, 10)
	go readSerialLoop(port, angleChan)

	for angle := range angleChan {
		loopStart := time.Now()

		serial_start := time.Now()
		// 시리얼 읽기는 readSerialLoop에서 하므로 패스
		fmt.Printf("1. 시리얼 통신 시간: %v\n", time.Since(serial_start))

		fmt.Println("받은 데이터 (Angle):", angle)

		quantized_angle := math.Round((1 / r) * angle)
		encode := []int64{int64(quantized_angle)}

		encode_start := time.Now()
		encoder.Encode(encode, pt_angle)
		ct_angle, _ := encryptor.EncryptNew(pt_angle)
		serialized_angle, err := ct_angle.MarshalBinary()
		if err != nil {
			fmt.Println("암호문 직렬화 실패:", err)
			continue
		}
		fmt.Printf("2. 암호화 및 데이터 변환 시간: %v\n", time.Since(encode_start))

		com_start := time.Now()
		_, err = conn.Write(serialized_angle)
		if err != nil {
			fmt.Println("TCP 전송 실패:", err)
			break
		}
		response, err := com_utils.ReadFullData(conn, 131406)
		if err != nil {
			fmt.Println("응답 수신 실패:", err)
			break
		}
		fmt.Printf("3. 암호 데이터의 TCP 송수신 시간: %v\n", time.Since(com_start))

		dec_start := time.Now()
		ct_U := rlwe.NewCiphertext(params, params.MaxLevel())
		err = ct_U.UnmarshalBinary(response)
		if err != nil {
			fmt.Println("응답 역직렬화 실패:", err)
			break
		}
		dec_U := decryptor.DecryptNew(ct_U)
		decoded := make([]int64, 1)
		encoder.Decode(dec_U, decoded)
		pwm_u := float64(decoded[0]) * r
		fmt.Printf("4. 복호화 및 데이터 변환 시간: %v\n", time.Since(dec_start))

		fmt.Println("제어 결과:", pwm_u)

		serial_2_start := time.Now()
		_, err = port.Write([]byte(fmt.Sprintf("%d,%d\n", int(pwm_u), 1)))
		if err != nil {
			fmt.Println("시리얼 전송 실패:", err)
			break
		}
		fmt.Printf("5. 시리얼 송신 시간: %v\n", time.Since(serial_2_start))

		fmt.Printf("총 루프 처리 시간: %v\n", time.Since(loopStart))
		fmt.Println("-----------------------------------")
	}
}

func setupSerial() (serial.Port, error) {
	mode := &serial.Mode{BaudRate: 115200}
	port, err := serial.Open("/dev/ttyACM0", mode)
	if err != nil {
		return nil, err
	}
	fmt.Println("포트 연결됨. 아두이노 리셋 대기 중...")
	time.Sleep(1 * time.Second)
	return port, nil
}

func readSerialLoop(port serial.Port, out chan<- float64) {
	reader := bufio.NewReader(port)
	defer close(out)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("시리얼 읽기 오류:", err)
			return
		}

		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, ",") {
			continue
		}

		angle, err := strconv.ParseFloat(line, 64)
		if err != nil {
			fmt.Println("파싱 실패:", err)
			continue
		}

		out <- angle
	}
}

func resetAndClosePort(port serial.Port) {
	fmt.Println("포트 정리 중...")
	port.ResetInputBuffer()
	port.ResetOutputBuffer()
	port.Close()
	time.Sleep(2 * time.Second)
}
