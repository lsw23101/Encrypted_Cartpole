// file: plant_rgsw_dual.go
package main

import (
	"Encrypted_Cartpole/com_utils"
	"bufio"
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"go.bug.st/serial"

	utils "github.com/CDSL-EncryptedControl/CDSL/utils"
	RLWE "github.com/CDSL-EncryptedControl/CDSL/utils/core/RLWE"
	"github.com/tuneinsight/lattigo/v6/core/rlwe"
)

const (
	// ===== TCP addresses =====
	// addrData = "127.0.0.1:9000" // 데이터 채널 (y 보내고 u 받기)
	// addrCtrl = "127.0.0.1:9001" // 제어 채널 (PAUSE/RESUME)
	addrData = "192.168.0.115:8080" // 컨트롤러 주소
	addrCtrl = "192.168.0.115:9000" // 컨트롤러 주소

	// ===== Arduino serial =====
	serialPort = "/dev/ttyACM0"
	baudRate   = 115200

	// ===== Loop pacing =====
	period = 0 * time.Millisecond // 0이면 최대 속도
)

// RLWE params & quant (컨트롤러와 동일해야 함)
const (
	logN   = 12
	logQ56 = 56
	logP51 = 51

	m = 1 // control input dimension
	p = 2 // measurement dimension

	s = 1.0 / 1.0
	L = 1.0 / 100000.0
	r = 1.0 / 10000.0
)

// "a,b" 형태에서 두 실수 관대 파싱
func parseTwoFloats(line string) (float64, float64, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return 0, 0, errors.New("empty line")
	}
	parts := strings.SplitN(line, ",", 3)
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("malformed: %q", line)
	}
	lhs := strings.TrimSpace(parts[0])
	rhs := strings.TrimSpace(parts[1])
	if lhs == "" || rhs == "" {
		return 0, 0, fmt.Errorf("empty token: %q", line)
	}
	a0, err0 := strconv.ParseFloat(lhs, 64)
	a1, err1 := strconv.ParseFloat(rhs, 64)
	if err0 != nil || err1 != nil {
		return 0, 0, fmt.Errorf("parse float failed: %v %v (line=%q)", err0, err1, line)
	}
	return a0, a1, nil
}

func main() {
	// ===== RLWE 세팅 =====
	params, _ := rlwe.NewParametersFromLiteral(rlwe.ParametersLiteral{
		LogN:    logN,
		LogQ:    []int{logQ56},
		LogP:    []int{logP51},
		NTTFlag: true,
	})
	ringQ := params.RingQ()

	// tau: >= max(m,p) 2의 거듭제곱
	maxDim := math.Max(float64(m), float64(p))
	tau := int(math.Pow(2, math.Ceil(math.Log2(maxDim))))

	// SecretKey 로드
	base := filepath.Join("..", "Offline_task", "enc_data", "rgsw")
	sk := new(rlwe.SecretKey)
	if err := com_utils.ReadRT(filepath.Join(base, "sk.dat"), sk); err != nil {
		log.Fatalf("load sk: %v", err)
	}
	encryptor := rlwe.NewEncryptor(params, sk)
	decryptor := rlwe.NewDecryptor(params, sk)

	// ===== TCP 연결 (데이터 채널) =====
	connData, err := net.Dial("tcp", addrData)
	if err != nil {
		log.Fatalf("tcp dial (data): %v", err)
	}
	defer connData.Close()
	rbufData := bufio.NewReader(connData)
	wbufData := bufio.NewWriter(connData)

	// ===== TCP 연결 (제어 채널) =====
	connCtrl, err := net.Dial("tcp", addrCtrl)
	if err != nil {
		log.Fatalf("tcp dial (ctrl): %v", err)
	}
	defer connCtrl.Close()
	rbufCtrl := bufio.NewReader(connCtrl)

	fmt.Println("[Plant] Connected to controller (data:", addrData, " ctrl:", addrCtrl, ")")

	// ===== 시리얼 오픈 =====
	mode := &serial.Mode{BaudRate: baudRate}
	port, err := serial.Open(serialPort, mode)
	if err != nil {
		log.Fatalf("serial open: %v", err)
	}
	defer port.Close()
	// 입력 버퍼 드레인 (가능 시)
	if rbuf, ok := port.(interface{ ResetInputBuffer() error }); ok {
		_ = rbuf.ResetInputBuffer()
	}
	sc := bufio.NewScanner(port)
	sc.Buffer(make([]byte, 0, 256), 1024)
	fmt.Println("[Plant] Serial opened:", serialPort, baudRate)

	// ===== 실행/정지 플래그 (atomic) =====
	var paused int32 = 0

	// ===== 제어 채널 goroutine =====
	go func() {
		for {
			msg, err := rbufCtrl.ReadString('\n')
			if err != nil {
				log.Println("[Plant] control channel closed:", err)
				// 제어 채널이 끊기면 안전상 정지 전환 & r 전송
				if atomic.SwapInt32(&paused, 1) == 0 {
					_, _ = port.Write([]byte("r\n"))
					fmt.Println("[Plant] CTRL closed → force PAUSE (sent 'r')")
				}
				return
			}
			switch msg {
			case "[CTRL]PAUSE\n":
				if atomic.SwapInt32(&paused, 1) == 0 {
					fmt.Println("[Plant] Received PAUSE → pausing loop")
					_, _ = port.Write([]byte("r\n"))
				}
			case "[CTRL]RESUME\n":
				if atomic.SwapInt32(&paused, 0) == 1 {
					fmt.Println("[Plant] Received RESUME → resuming loop")
					_, _ = port.Write([]byte("r\n"))
				}
			default:
				// 무시
			}
		}
	}()

	iter := 0
	for {
		// 1) 아두이노에서 한 줄 읽기 (y0,y1)
		if !sc.Scan() {
			if err := sc.Err(); err != nil {
				log.Printf("[Plant] Serial scan error: %v", err)
			} else {
				log.Printf("[Plant] Serial EOF")
			}
			break
		}
		line := sc.Text()

		// 일시정지 상태에서는 버퍼가 넘치지 않도록 라인은 소비하되, 네트워크 교환은 생략
		if atomic.LoadInt32(&paused) == 1 {
			// 필요하면 여기서 u=0을 보내고 싶을 수 있지만, 요구사항대로 PAUSE 시에는 r만 보냄(토글 1회).
			time.Sleep(50 * time.Millisecond)
			continue
		}

		// 2) y 파싱
		y0, y1, err := parseTwoFloats(line)
		if err != nil {
			log.Printf("[Plant] skip bad line: %v", err)
			continue
		}
		y := []float64{y0, y1}

		// ★ 로그 추가 (여기!)
		fmt.Printf("[Plant] y from Arduino = [%.6f, %.6f]\n", y0, y1)

		// 3) y 양자화 & 암호화
		yBar := utils.RoundVec(utils.ScalVecMult(1.0/r, y))
		yCtPack := RLWE.EncPack(yBar, tau, 1.0/L, *encryptor, ringQ, params)

		// 4) 컨트롤러(데이터 채널)로 y 암호문 전송
		if _, err := yCtPack.WriteTo(wbufData); err != nil {
			log.Printf("[Plant] Write yCtPack err at iter %d: %v (stop)", iter, err)
			break
		}
		if err := wbufData.Flush(); err != nil {
			log.Printf("[Plant] Flush err at iter %d: %v (stop)", iter, err)
			break
		}
		// fmt.Printf("[Plant] iter=%d sent y=[%.6f %.6f]\n", iter, y0, y1)

		// 5) u 암호문 수신
		uCtPack := new(rlwe.Ciphertext)
		if _, err := uCtPack.ReadFrom(rbufData); err != nil {
			log.Printf("[Plant] Read uCtPack err at iter %d: %v (stop)", iter, err)
			break
		}

		// 6) u 복호화 & 스케일 복원
		uVec := RLWE.DecUnpack(uCtPack, m, tau, *decryptor, r*s*s*L, ringQ, params)
		u := 0.0
		if len(uVec) > 0 {
			u = uVec[0]
		}

		// ★ 로그 추가 (여기!)
		fmt.Printf("[Plant] u after decrypt/scale = %.6f\n", u)

		// 7) 아두이노로 u 전송 (실행 중일 때만)
		if _, err := port.Write([]byte(fmt.Sprintf("%.6f\n", u))); err != nil {
			log.Printf("[Plant] Serial write err at iter %d: %v", iter, err)
			break
		}

		// 로그(원하면 주석 해제)
		// fmt.Printf("[Plant] iter=%d | y=[%.6f %.6f] -> u=%.6f\n", iter, y0, y1, u)

		iter++
		if period > 0 {
			time.Sleep(period)
		}
	}

	fmt.Println("[Plant] Stopped.")
}
