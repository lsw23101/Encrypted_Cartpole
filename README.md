Capstone_encryted_control
=============
2025 Capstone project repository

Description
====
<ardu_to_rasp, rasp_to_pc, pc_to_rasp>
- 일단 PID를 realization 한 연산에 대하여 통신 구현

<plant_rgsw.go and controller_rgsw.go>
- 앞서 구한 PID 상태공간 실현한 것 암호화 연산 나누기 성공
- 재암호화의 필요가 없으므로 13ms 내외가 걸림 (RLWE보다 장점)
- (중요!) Lattigo에서는 WriteTo랑 ReadFrom 이라는 함수를 지원하여 버퍼에 바로 담고 읽을 수 있었음
  => 기존에 직렬화하고 바이트 수만큼 버퍼 나누고 이러는 작업이 필요 x

~~<plant.go and controller.go>~~
~~- Lattigo CDSL 라이브러리 두개로 나눈거 돌려보는 실행파일~~
~~- 현재 한 루프에 30ms 이내 (프린트 문을 빼도 비슷함)~~
~~- 한번씩 통신 오류 발생 (+ 한번씩 50ms 넘는 루프타임)~~
~~- com_utils : 파일 읽고 쓰기 관련 함수~~



Requirment
=============
Go 설치하기

인터넷 연결 > ifconfig 로 ip 확인 
현재 컨트롤러용 pc 는 192.168.0.115 // 카트폴 : 192.168.0.30
or
127.0.0.1 로 설정해서 한 컴퓨터로 통신 시뮬레이션 돌려보기
(이렇게 했을 시에는 통신 오류 X)

Preliminary
===
~~1. RLWE
2. ARX form controller~~
1. RGSW (New!)
2. Discrete PID
3. TCP communicate


Usage
=============



```
git clone https://github.com/lsw23101/Encrypted_Cartpole
```





plant와 controller 코드에서 ip 설정

<terminal 1, 라즈베리파이>
```
cd ~/Rasberry
```

// 암호 상태공간 연산 통신
```
go run plant_rgsw.go 
```
// 아두이노와 통신 파일
```
go run rasp_to_pc.go 
```
// 아두이노에서 받아서 pc와 TCP 통신까지 연삲
```
go run pc_to_rasp.go 
```

<terminal 2, 서버 PC>
```
cd ~/PC
```

// 암호 상태공간 연산 통신
```
go run controller_rgsw.go
```
// 라즈베리 PID 연산 결과 통신 파일
```
go run pc_to_rasp.go 
```

Todo
====
RGSW로 상태공간 모델 연산으로 변경해야할듯 <- 완료

plant_rgsw.go 파일에서 출력으로 넣는 
y := []float64{0.001, 0.001} // 필요 시 시간에 따라 바꿔도 됨
위 코드 한줄을

adrdo_to_rasp.go 파일 바탕으로
아두이노에서 받아오는 출력 값으로 변경하면 끝

코드 정리

Offline task에서 파라미터 줄여서 세팅 가능할 것 같음 (F G H 행렬이 이미 정수...)

공격 시나리오 세팅..
PID 계수를 암호화 해야 하는 이유...
1. 현재는 PID라 의미가 적지만 모델을 아는 상황에서는 더욱 필요하다 (이번 프로젝트는 모델없이 제어해서 아쉬운 상황)
2. 말을 잘 꾸며내서 의미 담기 (e.g. 클라우드 서버는 이게 PID 연산인지 STATE SPACE 연산인지 조차도 모르게 하는 데에 의의가 있다. 즉, 어떤 연산을 하는지 아예 모른 상태로 서버는 돌아간다)

