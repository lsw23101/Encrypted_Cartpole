// file: plant_rgsw_dual.go
package main

import (
    "Encrypted_Cartpole/com_utils"
    "bufio"
    "fmt"
    "io"
    "log"
    "math"
    "net"
    "os"
    "path/filepath"
    "sync/atomic"
    "time"

    utils "github.com/CDSL-EncryptedControl/CDSL/utils"
    RLWE "github.com/CDSL-EncryptedControl/CDSL/utils/core/RLWE"
    "github.com/tuneinsight/lattigo/v6/core/rlwe"
)

const (
    addrData = "127.0.0.1:9000" // 데이터 채널
    addrCtrl = "127.0.0.1:9001" // 제어 채널
    period   = 0 * time.Millisecond
)

func main() {
    // ===== RLWE 세팅 =====
    params, _ := rlwe.NewParametersFromLiteral(rlwe.ParametersLiteral{
        LogN: 12, LogQ: []int{56}, LogP: []int{51}, NTTFlag: true,
    })
    ringQ := params.RingQ()

    m, p := 1, 2
    s := 1 / 1.0
    L := 1 / 100000.0
    r := 1 / 10000.0
    _ = s

    maxDim := math.Max(float64(m), float64(p))
    tau := int(math.Pow(2, math.Ceil(math.Log2(maxDim))))

    base := filepath.Join("..", "Offline_task", "enc_data", "rgsw")
    sk := new(rlwe.SecretKey)
    _ = com_utils.ReadRT(filepath.Join(base, "sk.dat"), sk)
    encryptor := rlwe.NewEncryptor(params, sk)
    decryptor := rlwe.NewDecryptor(params, sk)

    // ===== TCP 연결 (데이터 + 제어) =====
    connData, _ := net.Dial("tcp", addrData)
    defer connData.Close()
    rbufData := bufio.NewReader(connData)
    wbufData := bufio.NewWriter(connData)

    connCtrl, _ := net.Dial("tcp", addrCtrl)
    defer connCtrl.Close()
    rbufCtrl := bufio.NewReader(connCtrl)

    fmt.Println("[Plant] Connected to controller (data:", addrData, " ctrl:", addrCtrl, ")")

    // ===== 아두이노 대신 Mock =====
    var port io.Writer = os.Stdout
    fmt.Println("[Plant] Mock mode: writing to Stdout instead of Arduino")

    // ===== paused flag (atomic) =====
    var paused int32 = 0

    // ===== 제어 채널 goroutine =====
    go func() {
        for {
            msg, err := rbufCtrl.ReadString('\n')
            if err != nil {
                log.Println("[Plant] control channel closed:", err)
                return
            }
            switch msg {
            case "[CTRL]PAUSE\n":
                atomic.StoreInt32(&paused, 1)
                fmt.Println("[Plant] Received PAUSE → pausing loop")
                port.Write([]byte("r\n"))
            case "[CTRL]RESUME\n":
                atomic.StoreInt32(&paused, 0)
                fmt.Println("[Plant] Received RESUME → resuming loop")
                port.Write([]byte("r\n"))
            }
        }
    }()

    // ===== 메인 루프 =====
    for it := 0; ; it++ {
        if atomic.LoadInt32(&paused) == 1 {
            time.Sleep(100 * time.Millisecond)
            continue
        }

        // 1) y 생성 (고정값)
        y := []float64{0.001, 0.001}
        yBar := utils.RoundVec(utils.ScalVecMult(1/r, y))
        yCtPack := RLWE.EncPack(yBar, tau, 1/L, *encryptor, ringQ, params)

        // 2) y 암호문 전송
        yCtPack.WriteTo(wbufData)
        wbufData.Flush()
        fmt.Printf("[Plant] iter=%d sent yCtPack\n", it)

        // 3) u 암호문 수신
        uCtPack := new(rlwe.Ciphertext)
        if _, err := uCtPack.ReadFrom(rbufData); err != nil {
            log.Printf("[Plant] Read uCtPack err at iter %d: %v (stop)", it, err)
            break
        }
        u := RLWE.DecUnpack(uCtPack, 1, tau, *decryptor, r*s*s*L, ringQ, params)
        fmt.Printf("[Plant] iter=%d u(decrypted)=%v\n", it, u)

        // 4) 아두이노에 u 전송 (Mock → Stdout)
        if len(u) > 0 {
            port.Write([]byte(fmt.Sprintf("%.6f\n", u[0])))
        }

        if period > 0 {
            time.Sleep(period)
        }
    }
}
