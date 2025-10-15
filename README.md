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
현재 컨트롤러용 pc 는 192.168.0.115 // 카트폴 : 192.168.0.30 // 노트북: 192.168.0.20
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
4. Time delayed system


Usage
=============

```
git clone https://github.com/lsw23101/Encrypted_Cartpole
```

plant와 controller 코드에서 ip 설정

<terminal 1, 라즈베리파이>
```
cd ~/Raspberry
```

// 암호 상태공간 연산 통신
```
go run test_enc_plant.go
```

<terminal 2, 서버 PC, 먼저 실행>
```
cd ~/PC
```

// 암호 상태공간 연산 통신
```
go run controller_rgsw.go
```
