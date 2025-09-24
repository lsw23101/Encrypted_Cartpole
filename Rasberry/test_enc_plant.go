// file: enc_plant.go
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
	"time"

	"go.bug.st/serial"

	utils "github.com/CDSL-EncryptedControl/CDSL/utils"
	RLWE "github.com/CDSL-EncryptedControl/CDSL/utils/core/RLWE"
	"github.com/tuneinsight/lattigo/v6/core/rlwe"
)

// ===== 사용자 환경에 맞게 조정 =====
const (
	// addr       = "192.168.0.115:8080" // 컨트롤러 주소
	addr       = "127.0.0.1:9000" // 컨트롤러 주소
	serialPort = "/dev/ttyACM0"
	baudRate   = 115200

	// RLWE params (컨트롤러와 동일해야 함)
	logN   = 12
	logQ56 = 56
	logP51 = 51

	// 차원
	m = 1 // control input dimension
	p = 2 // measurement dimension

	// 양자화 스케일 (컨트롤러와 동일)
	s = 1.0 / 1.0
	L = 1.0 / 100000.0
	r = 1.0 / 10000.0
)

// 루프 주기 (원하면 조정: 0이면 최대 속도)
var period = 0 * time.Millisecond

// "a,b" 형태에서 두 실수를 관대하게 파싱
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

// 컨트롤러에서 토글(r/R) 신호가 도착했는지 확인하고 처리할지 결정
// true를 반환하면: r/R 한 줄을 소비했고, 이번 사이클은 암호문 읽기 없이 넘어가면 됨.
func readControllerToggleIfAny(rbuf *bufio.Reader) (bool, error) {
	// 최소 1바이트가 올 때까지 블록됨 (어차피 다음은 u암호문 또는 r라인이어야 함)
	bs, err := rbuf.Peek(1)
	if err != nil {
		return false, err
	}
	b := bs[0]
	// r/R로 시작하면 토글 명령으로 간주 (텍스트 라인)
	if b == 'r' || b == 'R' {
		// 줄 전체 소비
		_, err := rbuf.ReadString('\n')
		if err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
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
	// 입력 버퍼 드레인 (지원되는 경우)
	if r, ok := port.(interface{ ResetInputBuffer() error }); ok {
		_ = r.ResetInputBuffer()
	}
	sc := bufio.NewScanner(port)
	sc.Buffer(make([]byte, 0, 256), 1024)
	fmt.Println("[Combined] Serial opened:", serialPort, baudRate)

	paused := false
iter := 0
for {
    // 1) Arduino에서 한 줄 읽기
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
    y := []float64{y0, y1}

    // 양자화 -> EncPack
    yBar := utils.RoundVec(utils.ScalVecMult(1.0/r, y))
    yCtPack := RLWE.EncPack(yBar, tau, 1.0/L, *encryptor, ringQ, params)

    // 컨트롤러로 암호문 송신
    if _, err := yCtPack.WriteTo(wbuf); err != nil {
        log.Printf("[Combined] Write yCtPack err at iter %d: %v", iter, err)
        break
    }
    if err := wbuf.Flush(); err != nil {
        log.Printf("[Combined] Flush err at iter %d: %v", iter, err)
        break
    }

    // 🔎 2) 컨트롤러 토글 신호 확인
    if toggle, _ := readControllerToggleIfAny(rbuf); toggle {
        paused = !paused
        if paused {
            fmt.Println("[Combined] Received PAUSE → stop loop, send u=0")
        } else {
            fmt.Println("[Combined] Received RESUME → resume loop")
        }
    }

    if paused {
        // 멈춘 상태에서는 u=0만 아두이노로 보냄
		if _, err := port.Write([]byte("r\n")); err != nil {
			log.Printf("[Combined] Serial send 'r' err at iter %d: %v", iter, err)
			break
		}
        time.Sleep(100 * time.Millisecond)
        continue
    }

    // 3) 컨트롤러에서 제어입력 암호문 u 수신
    uCtPack := new(rlwe.Ciphertext)
    if _, err := uCtPack.ReadFrom(rbuf); err != nil {
        log.Printf("[Combined] Read uCtPack err at iter %d: %v", iter, err)
        break
    }

    // 4) 복호 & 스케일 되돌림
    uVec := RLWE.DecUnpack(uCtPack, m, tau, *decryptor, r*s*s*L, ringQ, params)
    u := 0.0
    if len(uVec) > 0 {
        u = uVec[0]
    }

    // 5) Arduino로 제어입력 송신
    if _, err := port.Write([]byte(fmt.Sprintf("%.6f\n", u))); err != nil {
        log.Printf("[Combined] Serial write err at iter %d: %v", iter, err)
        break
    }

    fmt.Printf("[Combined] iter=%d | y=[%.6f %.6f] -> u=%.6f\n", iter, y0, y1, u)

    iter++
    if period > 0 {
        time.Sleep(period)
    }
}
	fmt.Println("[Combined] Stopped.")
}
