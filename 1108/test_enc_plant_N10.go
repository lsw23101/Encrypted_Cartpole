// file: enc_plant_compare.go
package main

import (
	"Encrypted_Cartpole/com_utils"
	"bufio"
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"go.bug.st/serial"

	utils "github.com/CDSL-EncryptedControl/CDSL/utils"
	RLWE "github.com/CDSL-EncryptedControl/CDSL/utils/core/RLWE"
	RLWEpkg "github.com/tuneinsight/lattigo/v6/core/rlwe"
)

// ===== 사용자 환경 설정 =====
const (
	addr       = "192.168.0.115:8080" // TCP 컨트롤러 주소
	serialPort = "/dev/ttyACM0"
	baudRate   = 115200

	// RLWE params
	logN = 10
	logQ = 56
	logP = 51

	// 차원
	n = 4
	m = 1
	p = 2

	// 양자화 스케일
	s = 1.0 / 10.0
	L = 1.0 / 10000.0
	r = 1.0 / 1000.0
)

// PID 계수

	const (
		Kp = 32.0
		Ki = 2.5
		Kd = 42.0

		Lp = 30.0
		Li = 0.5
		Ld = 3.0
	)

// 안전 임계치
const (
	angleLimit    = 40.0  // |angle| > 40 → u=0
	positionLimit = 200.0 // |position| > 200 → u=0
)

// 리포트 파라미터
const (
	REPORT_EVERY_FRAMES = 200 // 몇 프레임마다 요약 리포트
	ACT_WINDOW_US       = 20000
	LARGE_UDIFF_THRESH  = 20.0 // uLocal vs uRemote 차이가 큰 경우 경고
)

// ---- 유틸: "a,b" 파싱 ----
func parseTwoFloats(line string) (float64, float64, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return 0, 0, errors.New("empty line")
	}
	parts := strings.SplitN(line, ",", 3)
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("malformed: %q", line)
	}
	a0, err0 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	a1, err1 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err0 != nil || err1 != nil {
		return 0, 0, fmt.Errorf("parse float failed: %v %v (line=%q)", err0, err1, line)
	}
	return a0, a1, nil
}

func main() {
	// ===== RLWE 세팅 =====
	params, _ := RLWEpkg.NewParametersFromLiteral(RLWEpkg.ParametersLiteral{
		LogN:    logN,
		LogQ:    []int{logQ},
		LogP:    []int{logP},
		NTTFlag: true,
	})
	ringQ := params.RingQ()

	maxDim := math.Max(math.Max(float64(n), float64(m)), float64(p))
	tau := int(math.Pow(2, math.Ceil(math.Log2(maxDim))))

	base := filepath.Join("..", "Offline_task", "enc_data", "rgsw_for_N10")
	sk := new(RLWEpkg.SecretKey)
	if err := com_utils.ReadRT(filepath.Join(base, "sk.dat"), sk); err != nil {
		log.Fatalf("load sk: %v", err)
	}
	encryptor := RLWEpkg.NewEncryptor(params, sk)
	decryptor := RLWEpkg.NewDecryptor(params, sk)

	// ===== TCP 연결 =====
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Fatalf("tcp dial: %v", err)
	}
	defer conn.Close()
	rbuf := bufio.NewReader(conn)
	wbuf := bufio.NewWriter(conn)
	fmt.Println("[INFO] Connected to controller:", addr)

	// ===== 시리얼 오픈 =====
	mode := &serial.Mode{BaudRate: baudRate}
	port, err := serial.Open(serialPort, mode)
	if err != nil {
		log.Fatalf("serial open: %v", err)
	}
	defer port.Close()
	sc := bufio.NewScanner(port)
	sc.Buffer(make([]byte, 0, 256), 1024)
	fmt.Println("[INFO] Serial opened:", serialPort, baudRate)

	// ===== 신호 처리 =====
	stopSig := make(chan os.Signal, 1)
	signal.Notify(stopSig, os.Interrupt, syscall.SIGTERM)
	quit := make(chan struct{})
	go func() {
		<-stopSig
		fmt.Println("\n[SIGNAL] Interrupt received — graceful stop after current iteration...")
		close(quit)
	}()

	// ===== 컨트롤러/상태 =====
	var (
		C = []float64{Ki, -Kd, Li, -Ld}
		D = []float64{Kp + Ki + Kd, Lp + Li + Ld}

		state = []float64{0, 0, 0, 0}
		y     = []float64{0, 0}
	)

	// ===== 리포트 집계 변수 =====
	var (
		frames             = 0
		lastYLine          = ""   // 중복의심 체크용
		dupYApprox         = 0    // 같은 문자열 y가 연달아 들어온 횟수
		sumLoopMs   float64 = 0.0 // y 수신 간격 평균
		minLoopMs   float64 = 1e9
		maxLoopMs   float64 = 0.0
		lastTime             time.Time

		sumTurnUS int64 = 0   // y 수신→u 송신까지
		maxTurnUS int64 = 0
		lateOrMiss      = 0   // 20ms 초과 회신 횟수

		sumTcpRTTms float64 = 0.0
		maxTcpRTTms float64 = 0.0

		serialWriteErr = 0
		tcpErr         = 0
		clampCount     = 0
		largeUDiff     = 0
	)

	printReport := func() {
		avgLoop := 0.0
		if frames > 0 {
			avgLoop = sumLoopMs / float64(frames)
		}
		avgTurn := 0.0
		if frames > 0 {
			avgTurn = float64(sumTurnUS) / float64(frames)
		}
		avgTcp := 0.0
		if frames > 0 {
			avgTcp = sumTcpRTTms / float64(frames)
		}

		fmt.Println("==== COMM/CTRL REPORT ====")
		fmt.Printf("frames=%d dupY_approx=%d\n", frames, dupYApprox)
		fmt.Printf("loop_ms avg=%.2f min=%.2f max=%.2f\n", avgLoop, minLoopMs, maxLoopMs)
		fmt.Printf("turnaround_us(avg y->u)=%.1f max=%d late_or_miss(>%d us)=%d\n",
			avgTurn, maxTurnUS, ACT_WINDOW_US, lateOrMiss)
		fmt.Printf("tcp_rtt_ms avg=%.3f max=%.3f errors=%d\n", avgTcp, maxTcpRTTms, tcpErr)
		fmt.Printf("serial_write_err=%d clamp=%d large_u_diff(>|%.1f|)=%d\n",
			serialWriteErr, clampCount, LARGE_UDIFF_THRESH, largeUDiff)
		fmt.Println("==========================")
	}

Loop:
	for {
		select {
		case <-quit:
			fmt.Println("[INFO] Stop requested before reading next sample.")
			break Loop
		default:
		}

		// 1) Arduino → y 수신
		if !sc.Scan() {
			if err := sc.Err(); err != nil {
				log.Printf("[ERR] Serial scan: %v", err)
			} else {
				log.Printf("[ERR] Serial EOF")
			}
			break
		}
		yLine := sc.Text()

		// 중복의심 체크
		if yLine == lastYLine {
			dupYApprox++
		}
		lastYLine = yLine

		y0, y1, err := parseTwoFloats(yLine)
		if err != nil {
			log.Printf("[WARN] skip bad line: %v", err)
			continue
		}
		y[0], y[1] = y0, y1

		// 루프 주기
		now := time.Now()
		if !lastTime.IsZero() {
			dtMs := float64(now.Sub(lastTime)) / 1e6
			sumLoopMs += dtMs
			if dtMs < minLoopMs {
				minLoopMs = dtMs
			}
			if dtMs > maxLoopMs {
				maxLoopMs = dtMs
			}
			// 필요하면 다음 줄 주석 해제해서 생생히 보기
			// fmt.Printf("[Loop] interval_ms=%.3f\n", dtMs)
		}
		lastTime = now

		// turnaround 측정 시작 (y수신→u송신)
		tY := time.Now()

		// 2) 로컬 u 계산
		uLocal := C[0]*state[0] + C[1]*state[1] + C[2]*state[2] + C[3]*state[3] +
			D[0]*y[0] + D[1]*y[1]

		// 3) 상태 업데이트
		state[0] += y[0]
		state[1] = y[0]
		state[2] += y[1]
		state[3] = y[1]

		// 4) y → 암호화 후 컨트롤러로 송신
		yBar := utils.RoundVec(utils.ScalVecMult(1.0/r, y))
		yCtPack := RLWE.EncPack(yBar, tau, 1.0/L, *encryptor, ringQ, params)

		tTcpStart := time.Now()
		if _, err := yCtPack.WriteTo(wbuf); err != nil {
			log.Printf("[ERR] TCP write yCtPack: %v", err)
			tcpErr++
			continue
		}
		if err := wbuf.Flush(); err != nil {
			log.Printf("[ERR] TCP flush: %v", err)
			tcpErr++
			continue
		}

		uCtPack := new(RLWEpkg.Ciphertext)
		if _, err := uCtPack.ReadFrom(rbuf); err != nil {
			log.Printf("[ERR] TCP read uCtPack: %v", err)
			tcpErr++
			continue
		}
		tcpRTTms := float64(time.Since(tTcpStart)) / 1e6
		sumTcpRTTms += tcpRTTms
		if tcpRTTms > maxTcpRTTms {
			maxTcpRTTms = tcpRTTms
		}
		// 필요하면 자세히:
		// fmt.Printf("[Latency] tcp_rtt_ms=%.3f\n", tcpRTTms)

		// 5) 복호화
		uVec := RLWE.DecUnpack(uCtPack, m, tau, *decryptor, r*s*s*L, ringQ, params)
		uRemote := 0.0
		if len(uVec) > 0 {
			uRemote = uVec[0]
		}

		// 6) 비교
		uDiff := uLocal - uRemote
		if math.Abs(uDiff) > LARGE_UDIFF_THRESH {
			largeUDiff++
			fmt.Printf("[WARN] large u diff: uLocal=%.3f uRemote=%.3f diff=%.3f\n", uLocal, uRemote, uDiff)
		}

		// 7) 안전 로직
		uOut := uRemote
		// clamped := false
		//if math.Abs(y[0]) > angleLimit || math.Abs(y[1]) > positionLimit {
		//	uOut = 0.0
		//	clamped = true
		//	clampCount++
		//	fmt.Printf("[SAFE] clamp u=0 (|ang|=%.2f |pos|=%.2f)\n", math.Abs(y[0]), math.Abs(y[1]))
		//}

		// 8) turnaround 끝 — u 송신
		turnUS := time.Since(tY).Microseconds()
		sumTurnUS += turnUS
		if turnUS > maxTurnUS {
			maxTurnUS = turnUS
		}
		if turnUS > ACT_WINDOW_US {
			lateOrMiss++
			fmt.Printf("[MISS?] y->u turnaround_us=%d (> %d)\n", turnUS, ACT_WINDOW_US)
		}

		if _, err := port.Write([]byte(fmt.Sprintf("%.6f\n", uOut))); err != nil {
			log.Printf("[ERR] Serial write: %v", err)
			serialWriteErr++
			continue
		}

		frames++
		if frames%REPORT_EVERY_FRAMES == 0 {
			printReport()
		}

		// 종료 신호 확인
		select {
		case <-quit:
			fmt.Println("[INFO] Stop requested by signal.")
			break Loop
		default:
		}
	}

	// 마지막 리포트
	fmt.Println("[INFO] Stopped. Final report:")
	fmt.Println("--------------------------------")
	// 한 번 더 출력
	{
		avgLoop := 0.0
		if frames > 0 {
			avgLoop = sumLoopMs / float64(frames)
		}
		avgTurn := 0.0
		if frames > 0 {
			avgTurn = float64(sumTurnUS) / float64(frames)
		}
		avgTcp := 0.0
		if frames > 0 {
			avgTcp = sumTcpRTTms / float64(frames)
		}
		fmt.Printf("frames=%d dupY_approx=%d\n", frames, dupYApprox)
		fmt.Printf("loop_ms avg=%.2f min=%.2f max=%.2f\n", avgLoop, minLoopMs, maxLoopMs)
		fmt.Printf("turnaround_us(avg y->u)=%.1f max=%d late_or_miss(>%d us)=%d\n",
			avgTurn, maxTurnUS, ACT_WINDOW_US, lateOrMiss)
		fmt.Printf("tcp_rtt_ms avg=%.3f max=%.3f errors=%d\n", avgTcp, maxTcpRTTms, tcpErr)
		fmt.Printf("serial_write_err=%d clamp=%d large_u_diff(>|%.1f|)=%d\n",
			serialWriteErr, clampCount, LARGE_UDIFF_THRESH, largeUDiff)
	}
	fmt.Println("--------------------------------")
}
