// file: controller_rgsw_dual.go
package main

import (
    "Encrypted_Cartpole/com_utils"
    "bufio"
    "fmt"
    "log"
    "math"
    "net"
    "os"
    "path/filepath"
    "time"

    RGSW "github.com/CDSL-EncryptedControl/CDSL/utils/core/RGSW"
    RLWE "github.com/CDSL-EncryptedControl/CDSL/utils/core/RLWE"
    "github.com/tuneinsight/lattigo/v6/core/rgsw"
    "github.com/tuneinsight/lattigo/v6/core/rlwe"
    "github.com/tuneinsight/lattigo/v6/ring"
)

const (
    addrData = "127.0.0.1:9000"
    addrCtrl = "127.0.0.1:9001"
    period   = 0 * time.Millisecond
)

func main() {
    // ===== Parameters =====
    params, _ := rlwe.NewParametersFromLiteral(rlwe.ParametersLiteral{
        LogN: 12, LogQ: []int{56}, LogP: []int{51}, NTTFlag: true,
    })
    ringQ := params.RingQ()

    n, m, p := 4, 1, 2
    s := 1 / 1.0
    L := 1 / 100000.0
    r := 1 / 10000.0
	_ = s
	_ = L
	_ = r


    maxDim := math.Max(math.Max(float64(n), float64(m)), float64(p))
    tau := int(math.Pow(2, math.Ceil(math.Log2(maxDim))))
    logn := int(math.Log2(float64(tau)))
    monomials := make([]ring.Poly, logn)
    for i := 0; i < logn; i++ {
        monomials[i] = ringQ.NewPoly()
        idx := params.N() - params.N()/(1<<(i+1))
        monomials[i].Coeffs[0][idx] = 1
        ringQ.MForm(monomials[i], monomials[i])
        ringQ.NTT(monomials[i], monomials[i])
    }

    base := filepath.Join("..", "Offline_task", "enc_data", "rgsw")
    recoveredX := new(rlwe.Ciphertext)
    _ = com_utils.ReadRT(filepath.Join(base, "xCtPack.dat"), recoveredX)
    ctF, _ := com_utils.LoadRGSWPack(base, "ctF")
    ctG, _ := com_utils.LoadRGSWPack(base, "ctG")
    ctH, _ := com_utils.LoadRGSWPack(base, "ctH")
    ctJ, _ := com_utils.LoadRGSWPack(base, "ctJ")

    rlk := new(rlwe.RelinearizationKey)
    _ = com_utils.ReadRT(filepath.Join(base, "rlk.dat"), rlk)
    gks, _ := com_utils.LoadGaloisKeys(base)

    evkRGSW := rlwe.NewMemEvaluationKeySet(rlk)
    evkRLWE := rlwe.NewMemEvaluationKeySet(rlk, gks...)
    evaluatorRGSW := rgsw.NewEvaluator(params, evkRGSW)
    evaluatorRLWE := rlwe.NewEvaluator(params, evkRLWE)

    zeroCt := rlwe.NewCiphertext(params, 1)

    // ===== TCP server (데이터 + 제어) =====
    lnData, _ := net.Listen("tcp", addrData)
    defer lnData.Close()
    connData, _ := lnData.Accept()
    defer connData.Close()
    rbufData := bufio.NewReader(connData)
    wbufData := bufio.NewWriter(connData)

    lnCtrl, _ := net.Listen("tcp", addrCtrl)
    defer lnCtrl.Close()
    connCtrl, _ := lnCtrl.Accept()
    defer connCtrl.Close()
    wbufCtrl := bufio.NewWriter(connCtrl)

    fmt.Println("[Controller] Listening on", addrData, "(data) and", addrCtrl, "(ctrl)")

    paused := false

    // ===== Keyboard goroutine =====
    go func() {
        scanner := bufio.NewScanner(os.Stdin)
        for scanner.Scan() {
            input := scanner.Text()
            if input == "r" {
                if paused {
                    wbufCtrl.WriteString("[CTRL]RESUME\n")
                    wbufCtrl.Flush()
                    fmt.Println("[Controller] Sent RESUME signal")
                    paused = false
                } else {
                    wbufCtrl.WriteString("[CTRL]PAUSE\n")
                    wbufCtrl.Flush()
                    fmt.Println("[Controller] Sent PAUSE signal")
                    paused = true
                }
            }
        }
    }()

    // ===== Main loop =====
    for it := 0; ; it++ {
        if paused {
            time.Sleep(100 * time.Millisecond)
            continue
        }

        // 1) receive y
        yCtPack := new(rlwe.Ciphertext)
        if _, err := yCtPack.ReadFrom(rbufData); err != nil {
            log.Printf("[Controller] Read yCtPack err at iter %d: %v (stop)", it, err)
            break
        }

        // 2) unpack
        xCt := RLWE.UnpackCt(recoveredX, n, tau, evaluatorRLWE, ringQ, monomials, params)
        yCt := RLWE.UnpackCt(yCtPack, p, tau, evaluatorRLWE, ringQ, monomials, params)

        // 3) compute u = Hx + Jy
        uCtPack := RGSW.MultPack(xCt, ctH, evaluatorRGSW, ringQ, params)
        JyCt := RGSW.MultPack(yCt, ctJ, evaluatorRGSW, ringQ, params)
        uCtPack = RLWE.Add(uCtPack, JyCt, zeroCt, params)

        // 4) send u
        if _, err := uCtPack.WriteTo(wbufData); err != nil {
            log.Printf("[Controller] Write uCtPack err at iter %d: %v (stop)", it, err)
            break
        }
        wbufData.Flush()

        // 5) update x
        FxCt := RGSW.MultPack(xCt, ctF, evaluatorRGSW, ringQ, params)
        GyCt := RGSW.MultPack(yCt, ctG, evaluatorRGSW, ringQ, params)
        recoveredX = RLWE.Add(FxCt, GyCt, zeroCt, params)

        if it%50 == 0 {
            fmt.Printf("[Controller] iter=%d done\n", it)
        }

        if period > 0 {
            time.Sleep(period)
        }
    }
}
