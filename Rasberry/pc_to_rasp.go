package main

import (
	"bufio"
	"fmt"
	"go.bug.st/serial"
	"net"
	"strconv"
	"strings"
	"time"
)

var lastTime time.Time

// PID 계수
const (
	Kp = 34.0
	Ki = 4.0
	Kd = 42.0

	Lp = 40.0
	Li = 0.0
	Ld = 3.0
)

// 상태공간 행렬 (참고용)
var A = [4][4]float64{
	{1, 0, 0, 0},
	{0, 0, 0, 0},
	{0, 0, 1, 0},
	{0, 0, 0, 0},
}
var B = [4][2]float64{
	{1, 0},
	{1, 0},
	{0, 1},
	{0, 1},
}

// 제어기 출력 행렬
var C = []float64{Ki, -Kd, Li, -Ld}
var D = []float64{Kp + Ki + Kd, Lp + Li + Ld}

// 상태, 출력, 입력
var state = []float64{0, 0, 0, 0} // 4x1
var y = []float64{0, 0}           // 2x1
var u = 0.0                       // 1x1

func main() {
	// --- 아두이노 연결 ---
	mode := &serial.Mode{BaudRate: 115200}
	port, err := serial.Open("/dev/ttyACM0", mode) // 라즈베리 UART 포트
	if err != nil {
		fmt.Println("시리얼 연결 실패:", err)
		return
	}
	defer port.Close()
	reader := bufio.NewReader(port)
	fmt.Println("아두이노 연결됨")

	// --- TCP 서버 연결 ---
	conn, err := net.Dial("tcp", "192.168.0.115:8080") // PC 서버 IP
	if err != nil {
		fmt.Println("TCP 연결 실패:", err)
		return
	}
	defer conn.Close()
	tcpReader := bufio.NewReader(conn)
	fmt.Println("TCP 서버와 연결됨")

	// --- 메인 루프 ---
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("시리얼 읽기 실패:", err)
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, ",") {
			continue
		}

		// y 파싱
		parts := strings.Split(line, ",")
		if len(parts) != 2 {
			continue
		}
		y0, err0 := strconv.ParseFloat(parts[0], 64)
		y1, err1 := strconv.ParseFloat(parts[1], 64)
		if err0 != nil || err1 != nil {
			continue
		}

		// loop interval 체크
		now := time.Now()
		if !lastTime.IsZero() {
			fmt.Printf("Loop interval: %v\n", now.Sub(lastTime))
		}
		lastTime = now

		y[0] = y0
		y[1] = y1

		// --- 라즈베리 내부에서 u 계산 ---
		u = C[0]*state[0] + C[1]*state[1] + C[2]*state[2] + C[3]*state[3] +
			D[0]*y[0] + D[1]*y[1]

		// 상태 업데이트
		state[0] += y[0]
		state[1] = y[0]
		state[2] += y[1]
		state[3] = y[1]

		fmt.Printf("받은 y = [%.3f, %.3f], 로컬 계산 u = %.3f\n", y0, y1, u)

		// --- TCP 서버로 y 보내기 ---
		_, err = conn.Write([]byte(fmt.Sprintf("%.6f,%.6f\n", y0, y1)))
		if err != nil {
			fmt.Println("TCP 전송 실패:", err)
			break
		}

		// --- TCP 응답 수신 ---
		response, err := tcpReader.ReadString('\n')
		if err != nil {
			fmt.Println("TCP 응답 수신 실패:", err)
			break
		}
		response = strings.TrimSpace(response)

		// 서버에서 받은 응답 → float64로 변환
		respVal, err := strconv.ParseFloat(response, 64)
		if err != nil {
			fmt.Println("응답 파싱 실패:", err)
			continue
		}

		// 라즈베리 계산 u와 서버 응답 비교
		diff := respVal - u
		fmt.Printf("서버 응답 u = %.3f, 로컬 u = %.3f, 차이 = %.6f\n", respVal, u, diff)

		// --- 아두이노로 송신 ---
		_, err = port.Write([]byte(response + "\n"))
		if err != nil {
			fmt.Println("아두이노로 전송 실패:", err)
			break
		}
		fmt.Println("아두이노로 보낸 제어입력:", response)
	}
}
