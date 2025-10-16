// file: enc_plant_compare.go
package main

import (
	"Encrypted_Cartpole/com_utils"
	"bufio"
	"encoding/csv"
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
	"github.com/tuneinsight/lattigo/v6/core/rlwe"
)

// ===== 사용자 환경 설정 =====
const (
	addr       = "192.168.0.115:8080" // TCP 컨트롤러 주소
	serialPort = "/dev/ttyACM0"
	baudRate   = 115200

	// RLWE params
	logN = 12
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
	Kd = 40.0

	Lp = 30.0
	Li = 0.1
	Ld = 3.0
)

// ===== 추가: 안전 임계치 & 루프 횟수 =====
const (
	angleLimit    = 40.0 // |angle| > 40 → u=0
	positionLimit = 50.0 // |position| > 50 → u=0
	maxIter       = 0    // 0=무한루프, 양수=그 횟수만큼만 실행
)

// 상태공간 행렬
var C = []float64{Ki, -Kd, Li, -Ld}
var D = []float64{Kp + Ki + Kd, Lp + Li + Ld}

var state = []float64{0, 0, 0, 0}
var y = []float64{0, 0}

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

// ---- CSV 저장 ----
func saveCSV(path string, rows [][]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{
		"iter", "t_ms",
		"y0_angle", "y1_position",
		"uLocal", "uRemote", "uOut", "uDiff",
		"loopIntervalMs", "tcpRttMs",
		"clamped",
	}
	if err := w.Write(header); err != nil {
		return err
	}
	for _, r := range rows {
		if err := w.Write(r); err != nil {
			return err
		}
	}
	return w.Error()
}

func boolTo01(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func main() {
	// ===== RLWE 세팅 =====
	params, _ := rlwe.NewParametersFromLiteral(rlwe.ParametersLiteral{
		LogN:    logN,
		LogQ:    []int{logQ},
		LogP:    []int{logP},
		NTTFlag: true,
	})
	ringQ := params.RingQ()

	maxDim := math.Max(math.Max(float64(n), float64(m)), float64(p))
	tau := int(math.Pow(2, math.Ceil(math.Log2(maxDim))))

	base := filepath.Join("..", "Offline_task", "enc_data", "rgsw_for_N12")
	sk := new(rlwe.SecretKey)
	if err := com_utils.ReadRT(filepath.Join(base, "sk.dat"), sk); err != nil {
		log.Fatalf("load sk: %v", err)
	}
	encryptor := rlwe.NewEncryptor(params, sk)
	decryptor := rlwe.NewDecryptor(params, sk)

	// ===== TCP 연결 =====
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Fatalf("tcp dial: %v", err)
	}
	defer conn.Close()
	rbuf := bufio.NewReader(conn)
	wbuf := bufio.NewWriter(conn)
	fmt.Println("[Combined] Connected to controller:", addr)

	// ===== 시리얼 오픈 =====
	mode := &serial.Mode{BaudRate: baudRate}
	port, err := serial.Open(serialPort, mode)
	if err != nil {
		log.Fatalf("serial open: %v", err)
	}
	defer port.Close()
	sc := bufio.NewScanner(port)
	sc.Buffer(make([]byte, 0, 256), 1024)
	fmt.Println("[Combined] Serial opened:", serialPort, baudRate)

	// ===== 신호 처리 (Ctrl+C / SIGTERM) =====
	stopSig := make(chan os.Signal, 1)
	signal.Notify(stopSig, os.Interrupt, syscall.SIGTERM)
	quit := make(chan struct{})
	go func() {
		<-stopSig
		fmt.Println("\n[Signal] Interrupt received. Will stop after current iteration and save CSV...")
		close(quit)
	}()

	// ===== 로깅 준비 =====
	startT := time.Now()
	csvPath := fmt.Sprintf("enc_plant_log_%s.csv", time.Now().Format("20060102_150405"))
	records := make([][]string, 0, 4096)
	fmt.Println("[CSV] Logging to:", csvPath)

	// 종료 시 CSV 저장(가능한 한 보장)
	defer func() {
		if len(records) == 0 {
			fmt.Println("[CSV] No data collected.")
			return
		}
		if err := saveCSV(csvPath, records); err != nil {
			log.Printf("[CSV] Save error: %v", err)
		} else {
			fmt.Printf("[CSV] Saved %d rows to %s\n", len(records), csvPath)
		}
	}()

	var lastTime time.Time
	iter := 0

Loop:
	for {
		// 신호로 중지 요청이 이미 들어왔고, 새 입력을 기다리기 싫다면 여기서도 체크 가능
		select {
		case <-quit:
			fmt.Println("[Combined] Stop requested before reading next sample.")
			break Loop
		default:
		}

		// 1) Arduino에서 y 읽기 (angle=y[0], position=y[1] 가정)
		if !sc.Scan() {
			if err := sc.Err(); err != nil {
				log.Printf("[Combined] Serial scan error: %v", err)
			} else {
				log.Printf("[Combined] Serial EOF")
			}
			break
		}
		line := sc.Text()
		y0, y1, err := parseTwoFloats(line)
		if err != nil {
			log.Printf("[Combined] skip bad line: %v", err)
			continue
		}
		y[0] = y0 // angle
		y[1] = y1 // position

		// 루프 주기 모니터링
		now := time.Now()
		intervalMs := 0.0
		if !lastTime.IsZero() {
			intervalMs = float64(now.Sub(lastTime)) / 1e6
			fmt.Printf("[Loop] interval: %.3f ms\n", intervalMs)
		}
		lastTime = now

		// 2) 로컬 제어 입력 계산
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

		// 🔹 TCP 왕복 시간 측정 시작
		tStart := time.Now()

		if _, err := yCtPack.WriteTo(wbuf); err != nil {
			log.Printf("[Combined] Write yCtPack err: %v", err)
			break
		}
		if err := wbuf.Flush(); err != nil {
			log.Printf("[Combined] Flush err: %v", err)
			break
		}

		// 컨트롤러 응답 수신
		uCtPack := new(rlwe.Ciphertext)
		if _, err := uCtPack.ReadFrom(rbuf); err != nil {
			log.Printf("[Combined] Read uCtPack err: %v", err)
			break
		}

		// 🔹 TCP 왕복시간
		rttMs := float64(time.Since(tStart)) / 1e6
		fmt.Printf("[Latency] TCP round-trip: %.3f ms\n", rttMs)

		// 5) 복호화 및 스케일 복원
		uVec := RLWE.DecUnpack(uCtPack, m, tau, *decryptor, r*s*s*L, ringQ, params)
		uRemote := 0.0
		if len(uVec) > 0 {
			uRemote = uVec[0]
		}

		// 6) 두 제어 입력 비교 출력
		uDiff := uLocal - uRemote
		fmt.Printf("[Compare] uLocal=%.6f | uRemote=%.6f | Δ=%.6f\n", uLocal, uRemote, uDiff)

		// 7) 안전 로직: |angle|>40 또는 |position|>200 이면 u=0
		angle := y[0]
		position := y[1]
		uOut := uRemote
		clamped := false
		if math.Abs(angle) > angleLimit || math.Abs(position) > positionLimit {
			uOut = 0.0
			clamped = true
			fmt.Printf("[SAFEGUARD] |angle|=%.3f, |position|=%.3f beyond (%.1f, %.1f) → u=0 sent.\n",
				math.Abs(angle), math.Abs(position), angleLimit, positionLimit)
		}

		// 8) 실제로 아두이노에 보낼 것은 uOut
		if _, err := port.Write([]byte(fmt.Sprintf("%.6f\n", uOut))); err != nil {
			log.Printf("[Combined] Serial write err: %v", err)
			break
		}

		// 9) 로깅 (CSV용)
		elapsedMs := float64(time.Since(startT)) / 1e6
		record := []string{
			strconv.Itoa(iter),
			fmt.Sprintf("%.3f", elapsedMs),
			fmt.Sprintf("%.6f", y[0]),
			fmt.Sprintf("%.6f", y[1]),
			fmt.Sprintf("%.6f", uLocal),
			fmt.Sprintf("%.6f", uRemote),
			fmt.Sprintf("%.6f", uOut),
			fmt.Sprintf("%.6f", uDiff),
			fmt.Sprintf("%.3f", intervalMs),
			fmt.Sprintf("%.3f", rttMs),
			boolTo01(clamped),
		}
		records = append(records, record)

		iter++

		// 10) 루프 종료 조건
		if maxIter > 0 && iter >= maxIter {
			fmt.Println("[Combined] Reached max iterations.")
			break
		}
		select {
		case <-quit:
			fmt.Println("[Combined] Stop requested by signal.")
			break Loop
		default:
		}
	}
	fmt.Println("[Combined] Stopped.")
}
