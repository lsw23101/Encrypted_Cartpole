package main

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

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

// 상태
var state = []float64{0, 0, 0, 0} // 4x1

var lastTime time.Time // 루프 인터벌 측정용

func main() {
	// 서버 오픈
	listener, err := net.Listen("tcp", "192.168.0.115:8080")
	if err != nil {
		fmt.Println("TCP 리스너 실패:", err)
		return
	}
	defer listener.Close()
	fmt.Println("상태공간 제어 서버 실행 중 (192.168.0.115:8080)")

	conn, err := listener.Accept()
	if err != nil {
		fmt.Println("연결 수락 실패:", err)
		return
	}
	defer conn.Close()
	fmt.Println("라즈베리파이 클라이언트 연결됨")

	reader := bufio.NewReader(conn)

	for {
		// y 수신
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("데이터 수신 실패:", err)
			break
		}
		line = strings.TrimSpace(line)
		parts := strings.Split(line, ",")
		if len(parts) != 2 {
			continue
		}

		y0, err0 := strconv.ParseFloat(parts[0], 64)
		y1, err1 := strconv.ParseFloat(parts[1], 64)
		if err0 != nil || err1 != nil {
			continue
		}
		y := []float64{y0, y1}

		// --- 루프 인터벌 체크 ---
		now := time.Now()
		if !lastTime.IsZero() {
			interval := now.Sub(lastTime)
			fmt.Printf("Loop interval: %v\n", interval)
		}
		lastTime = now
		// -----------------------

		// 제어 입력 u 계산 (u = Cx + Dy)
		u := C[0]*state[0] + C[1]*state[1] + C[2]*state[2] + C[3]*state[3] +
			D[0]*y[0] + D[1]*y[1]

		// 상태 업데이트
		state[0] += y[0]
		state[1] = y[0]
		state[2] += y[1]
		state[3] = y[1]

		// 응답 전송
		response := fmt.Sprintf("%.6f", u)
		_, err = conn.Write([]byte(response + "\n"))
		if err != nil {
			fmt.Println("응답 전송 실패:", err)
			break
		}

		fmt.Printf("받은 y = [%.3f, %.3f], 보낸 u = %.3f\n", y0, y1, u)
	}
}
