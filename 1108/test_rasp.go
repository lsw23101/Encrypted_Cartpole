// ================== RPi Go — 최소 통신(프레임ID 없음) + 15ms 대기 + u 회신 ==================
package main

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.bug.st/serial"
)

// ==== PID 계수 (필요시 조정) ====
const (
	Kp = 32.0
	Ki = 2.5
	Kd = 40.0

	Lp = 35.0
	Li = 0.7
	Ld = 10.0
)

// 제어기 행렬
var C = []float64{Ki, -Kd, Li, -Ld}
var D = []float64{Kp + Ki + Kd, Lp + Li + Ld}

// 상태, 출력, 입력
var state = []float64{0, 0, 0, 0} // 4x1 누산/보유 상태
var y = []float64{0, 0}
var u = 0.0

// ==== 통신/대기 설정 ====
const (
	BAUD       = 115200
	SERIAL_DEV = "/dev/ttyACM0"
	SLEEP_MS   = 15 // y→u 계산 후 고정 대기 시간
)

// 안전 한계 (간단 보호)
const (
	angleLimit = 40.0 // |angle| > 40 → u=0
)

func main() {
	mode := &serial.Mode{BaudRate: BAUD}
	port, err := serial.Open(SERIAL_DEV, mode)
	if err != nil {
		fmt.Println("serial open failed:", err)
		return
	}
	defer port.Close()

	reader := bufio.NewReader(port)
	fmt.Println("RPi controller started (no frame ID, 15ms wait, echo u only)")

	for {
		// 1) 아두이노에서 y 라인 수신: "y0,y1"
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("serial read error:", err)
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, ",")
		if len(parts) != 2 {
			// 배너/잡음 등은 무시
			continue
		}

		y0, err0 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		y1, err1 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err0 != nil || err1 != nil {
			continue
		}
		tRecv := time.Now()

		// 2) u 계산 (u = Cx + Dy)
		y[0], y[1] = y0, y1
		u = C[0]*state[0] + C[1]*state[1] + C[2]*state[2] + C[3]*state[3] +
			D[0]*y[0] + D[1]*y[1]

		// 간단 각도 보호
		if y[0] > angleLimit || y[0] < -angleLimit {
			u = 0
		}

		// 상태 업데이트
		state[0] += y[0]
		state[1] = y[0]
		state[2] += y[1]
		state[3] = y[1]

		// 3) 15ms 대기 후 회신
		time.Sleep(SLEEP_MS * time.Millisecond)
		if _, err := port.Write([]byte(fmt.Sprintf("%.6f\n", u))); err != nil {
			fmt.Println("serial write error:", err)
			continue
		}

		// 4) 출력: 턴어라운드 시간만 표시 (y 수신→u 송신까지)
		turnaroundMs := float64(time.Since(tRecv).Microseconds()) / 1000.0
		fmt.Printf("turnaround_ms=%.3f", turnaroundMs)
		fmt.Printf("y0=%.3f", y0)
		fmt.Printf("y1=%.3f", y1)
		fmt.Printf("u=%.3f\n", u)
	}
}
