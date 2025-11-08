// ================== RPi Go — 프레임ID 기반 디버그(중복/지연/드롭) + u 회신 ==================
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
	Kp = 34.0
	Ki = 2.5
	Kd = 34.0

	Lp = 30.0
	Li = 0.5
	Ld = 5.0
)

// 제어기 행렬
var C = []float64{Ki, -Kd, Li, -Ld}
var D = []float64{Kp + Ki + Kd, Lp + Li + Ld}

// 상태, 출력, 입력
var state = []float64{0, 0, 0, 0} // 4x1
var y = []float64{0, 0}
var u = 0.0

// ==== 디버그/측정 파라미터 ====
const (
	BAUD              = 115200
	SERIAL_DEV        = "/dev/ttyACM0"
	SLEEP_MS          = 15                 // y→u 계산 후 sleep
	ACT_WINDOW_US     = 20000              // 아두이노 창(참고)
	REPORT_EVERY_FRMS = 200                // 리포트 주기
)

func main() {
	mode := &serial.Mode{BaudRate: BAUD}
	port, err := serial.Open(SERIAL_DEV, mode)
	if err != nil {
		fmt.Println("시리얼 연결 실패:", err)
		return
	}
	defer port.Close()

	reader := bufio.NewReader(port)
	fmt.Println("라즈베리파이 제어기 시작(fid/dup/late/drop 디버그)")

	// 진단 통계
	var (
		lastFidSeen       = int64(-1)
		dupY              = 0 // 같은 fid y 중복 수신
		dropY             = 0 // fid 건너뜀
		ooY               = 0 // out-of-order (역순)
		frames            = 0

		sumTurnUS   int64 = 0
		maxTurnUS   int64 = 0
		lateOrMissU       = 0 // 20ms 초과 회신(아두이노 적용 실패 가능성 높음)

		lastPrint = time.Now()
	)

	for {
		// 1) 아두이노에서 y 라인 수신: "fid,angErr,posErr"
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("시리얼 읽기 실패:", err)
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, ",")
		if len(parts) != 3 {
			// 쉼표가 2개가 아니면 y 라인이 아님(배너 등) → 무시
			continue
		}

		// 파싱
		fid64, errF := strconv.ParseInt(parts[0], 10, 64)
		y0, err0 := strconv.ParseFloat(parts[1], 64)
		y1, err1 := strconv.ParseFloat(parts[2], 64)
		if errF != nil || err0 != nil || err1 != nil {
			continue
		}

		// 진단: 중복/드롭/역순
		if lastFidSeen >= 0 {
			if fid64 == lastFidSeen {
				dupY++
			} else if fid64 == lastFidSeen+1 {
				// 정상 증가
			} else if fid64 > lastFidSeen+1 {
				dropY += int(fid64 - (lastFidSeen + 1))
			} else if fid64 < lastFidSeen {
				ooY++
			}
		}
		lastFidSeen = fid64

		// 타임스탬프
		tY := time.Now()

		// 2) u 계산 (u = Cx + Dy)
		y[0], y[1] = y0, y1
		u = C[0]*state[0] + C[1]*state[1] + C[2]*state[2] + C[3]*state[3] +
			D[0]*y[0] + D[1]*y[1]

		// 간단한 각도 보호
		if y[0] > 40 || y[0] < -40 {
			u = 0
		}

		// 상태 업데이트
		state[0] += y[0]
		state[1] = y[0]
		state[2] += y[1]
		state[3] = y[1]

		// 3) 15ms 대기 + 회신 (fid와 함께)
		time.Sleep(SLEEP_MS * time.Millisecond)
		tSend := time.Now()
		turnUS := tSend.Sub(tY).Microseconds()
		if turnUS > maxTurnUS {
			maxTurnUS = turnUS
		}
		sumTurnUS += turnUS
		frames++

		// 20ms 초과면 아두이노 창 놓쳤을 가능성 높음
		if turnUS > ACT_WINDOW_US {
			lateOrMissU++
		}

		// 회신: "fid,u"
		out := fmt.Sprintf("%d,%.6f\n", fid64, u)
		_, _ = port.Write([]byte(out))

		// 4) 주기적 리포트 (표준 출력으로만, 아두이노와 무관)
		if frames%REPORT_EVERY_FRMS == 0 || time.Since(lastPrint) > 10*time.Second {
			avg := float64(sumTurnUS) / float64(frames)
			fmt.Println("==== COMM/RTT REPORT ====")
			fmt.Printf("frames=%d dupY=%d dropY=%d outOfOrder=%d\n", frames, dupY, dropY, ooY)
			fmt.Printf("turnaround_us avg=%.1f max=%d late_or_miss=%d (>%d us)\n",
				avg, maxTurnUS, lateOrMissU, ACT_WINDOW_US)
			fmt.Println("=========================")
			lastPrint = time.Now()
		}
	}
}
