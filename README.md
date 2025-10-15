Capstone_encryted_control
=============
2025 Capstone project repository

# 시간 지연 시스템
현재 샘플링 시간: 30ms 

하지만 직접항, feedthrough term 이 존재하기 때문에 시간 계획으로는 시간 지연을 없앨 수 없음
>> 플랜트에서 송신 - 제어기 연산 - 플랜트 수신 까지의 시간이 지연되어 제어입력이 들어감

y(t) 출력에 대하여 입력은 u(t + $\tau$)


# 제어기

출력 2개에 대하여 병렬 PID 사용
>> 상태공간으로 realization

이때 상태행렬은 diag([0 1 0 1]) >> 재암호화 필요 x

error growth는 closed loop stability로 제어
(||u|| < 0.1901)


# ToDo
1. PID fine tuning
2. 기구 꾸미기



# 결과
```
PS C:\Users\sang2\all\Encrypted_Cartpole\Pc> go run .\controller_rgsw.go
[Controller] Listening on 192.168.0.115:8080 ...  
[Controller][AVG over 500] recv=5.340 ms, unpack=2.213 ms, computeU=2.256 ms, send=0.098 ms, update=2.204 ms, total=12.113 ms | bytes avg y=64.3 KB, avg u=64.3 KB  
[Controller][AVG comm over 499 samples] send->nextRecv = 7.541 ms  
[Controller] Done. (r,s,L) = 0.0001 0.0001 0.001 | m = 1
```
send(Lattigo의 직렬화에 걸리는 비용), 역직렬화는 recv에 담김  

통신 : 8ms (u 보내고 다음 y 받는데 까지 걸리는 시간. 즉, 플랜트의 암호화 등의 시간도 포함)  
연산 : 7ms (unpack, compute u, state update 각 2.x ms)  

결론: 루프 시간 20ms로 설정도 넉넉하게 가능할 것으로 예상

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

개인 노트북(or 연구실 노트북)으로 돌려도 50ms 이내가 돌아간다면 시연 상황도 굿

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

