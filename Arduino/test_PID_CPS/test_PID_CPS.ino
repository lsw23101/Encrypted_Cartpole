// ================== Arduino — RPi 통신(115200 고정) + 20ms 지연 + 30ms 루프 + 타이밍 리포트 ==================
#include <Arduino.h>
#include <math.h>

// ---------------- 핀 설정 ----------------
const int MOTOR_DIR_PIN = 12;
const int MOTOR_PWM_PIN = 11;

const int encoderPinA   = 2;   // 인터럽트 핀 (A상)
const int encoderPinB   = 7;   // B상

volatile long encoderCount = 0;

// ---------------- 센서/보정 상수 ----------------
float  ADCvalue      = 0.0f;
float  currentAngle  = 0.0f;
const float ADCmin   = 104.0f;
const float ADCmax   = 919.0f;
const float ANGLE_OFFSET = 77.3f - 1.73f + 34.0f - 1.0f; // 사용 오프셋

// ---------------- 타겟 ----------------
double targetAngle    = 0.0;
double targetPosition = 0.0;

// ---------------- 출력(y) / 입력(u) ----------------
double y0 = 0.0, y1 = 0.0;   // y[0]=angleError, y[1]=positionError
double u_rx = 0.0;           // 이번 주기에 받은 u (있다면)
double u_last = 0.0;         // 이전 주기에 적용했던 u (백업용)

// ---------------- 타이밍 ----------------
unsigned long lastControlTime         = 0;
const unsigned long controlIntervalMs = 30;     // 전체 제어 주기 30ms
const unsigned long actuationDelayUs  = 20000;  // 액츄에이터 지연 20ms = 20000us
bool isRunning = true;

// ---------------- 타이밍 통계(링버퍼) ----------------
const uint16_t NSTAT = 200;              // 몇 프레임을 모아서 보고할지
uint32_t delay_us_hist[NSTAT];           // 적용 지연(us): apply_time - t_calc
uint32_t loop_us_hist[NSTAT];            // 루프 주기(us): t_calc - prev_t_calc
uint16_t stat_idx = 0;
uint32_t prev_t_calc = 0;
bool have_prev_t = false;
uint32_t u_miss_count = 0;               // 20ms 창에서 u 미수신 횟수

// ---------------- 유틸 ----------------
inline float getCartDistanceM() {
  // 엔코더 255 CPR 가정
  return (encoderCount * 2.0f * PI) / 255.0f;
}

inline void moveMotor(int pwm, bool forward) {
  digitalWrite(MOTOR_DIR_PIN, forward ? HIGH : LOW);
  analogWrite(MOTOR_PWM_PIN, pwm);
}

inline void stopMotor() {
  analogWrite(MOTOR_PWM_PIN, 0);
}

// ---------------- 엔코더 ISR ----------------
void updateEncoder() {
  bool A = digitalRead(encoderPinA);
  bool B = digitalRead(encoderPinB);
  if (A == B) encoderCount++;
  else        encoderCount--;
}

// ---------------- 시리얼 수신(u) 파싱 (완전 비블로킹) ----------------
// 라즈베리에서 '\n'으로 끝나는 실수 문자열 전송 가정 (예: "-12.345000\n")
bool tryReadU(double &u_out) {
  static char buf[32];
  static uint8_t idx = 0;

  while (Serial.available() > 0) {
    char c = Serial.read();
    if (c == '\r') continue; // CR 무시
    if (c == '\n') {
      buf[idx] = '\0';
      idx = 0;
      if (buf[0] == '\0') continue; // 빈 줄 무시
      u_out = atof(buf);
      return true;
    }
    if (idx < sizeof(buf) - 1) buf[idx++] = c;  // 넘치면 나머지는 버림
  }
  return false;
}

// ---------------- 설정 ----------------
void setup() {
  Serial.begin(115200);  // ★ 시리얼 속도 고정
  pinMode(MOTOR_DIR_PIN, OUTPUT);
  pinMode(MOTOR_PWM_PIN, OUTPUT);

  pinMode(encoderPinA, INPUT_PULLUP);
  pinMode(encoderPinB, INPUT_PULLUP);
  attachInterrupt(digitalPinToInterrupt(encoderPinA), updateEncoder, CHANGE);

  // 시작 배너(쉼표 없음: RPi 파서 혼선 방지)
  Serial.println("Arduino ready @115200, 30ms loop, 20ms delay");
}

// ---------------- 타이밍 리포트 ----------------
void printTimingReportAndReset() {
  // 통계 계산
  uint64_t sumDelay = 0, sumLoop = 0;
  uint32_t minDelay = 0xFFFFFFFFUL, maxDelay = 0;
  uint32_t minLoop  = 0xFFFFFFFFUL, maxLoop  = 0;
  uint16_t validLoopCount = 0;

  for (uint16_t i = 0; i < NSTAT; i++) {
    uint32_t d = delay_us_hist[i];
    sumDelay += d;
    if (d < minDelay) minDelay = d;
    if (d > maxDelay) maxDelay = d;

    uint32_t l = loop_us_hist[i];
    if (l != 0) { // 첫 프레임은 0일 수 있음
      sumLoop += l;
      validLoopCount++;
      if (l < minLoop) minLoop = l;
      if (l > maxLoop) maxLoop = l;
    }
  }

  double avgDelay = (double)sumDelay / (double)NSTAT;
  double avgLoop  = validLoopCount ? (double)sumLoop / (double)validLoopCount : 0.0;

  // 쉼표 없이 요약 출력 (RPi가 무시)
  Serial.println("TIMING REPORT BEGIN");
  Serial.print("samples=");   Serial.println(NSTAT);
  Serial.print("delay_us avg="); Serial.print(avgDelay, 1);
  Serial.print(" min=");        Serial.print(minDelay);
  Serial.print(" max=");        Serial.println(maxDelay);
  Serial.print("loop_us  avg="); Serial.print(avgLoop, 1);
  Serial.print(" min=");        Serial.print(minLoop == 0xFFFFFFFFUL ? 0 : minLoop);
  Serial.print(" max=");        Serial.println(maxLoop);
  Serial.print("u_miss=");      Serial.println(u_miss_count);
  Serial.println("TIMING REPORT END");

  // 리셋
  stat_idx = 0;
  have_prev_t = false;
  u_miss_count = 0;
}

// ---------------- 메인 루프 ----------------
void loop() {
  if (!isRunning) return;

  unsigned long now = millis();
  if (now - lastControlTime >= controlIntervalMs) {
    lastControlTime += controlIntervalMs;

    // 1) ADC -> 각도
    ADCvalue = analogRead(A1);
    currentAngle = (ADCvalue - ADCmin) * 360.0f / (ADCmax - ADCmin);
    currentAngle = constrain(currentAngle, 0.0f, 360.0f);

    // 오프셋 보정
    currentAngle += ANGLE_OFFSET;
    if (currentAngle < 0.0f)    currentAngle += 360.0f;
    if (currentAngle >= 360.0f) currentAngle -= 360.0f;

    // -180 ~ +180 변환
    float relativeAngle = currentAngle;
    if (relativeAngle > 180.0f) relativeAngle -= 360.0f;

    // 2) 오차 계산
    y0 = (double)targetAngle    - (double)relativeAngle;
    y1 = (double)targetPosition - (double)getCartDistanceM();

    // 3) ==== y 계산 시각 고정 ====
    uint32_t t_calc   = micros();
    uint32_t deadline = t_calc + (uint32_t)actuationDelayUs;

    // 4) ==== y 즉시 전송 ====
    Serial.print(y0, 3); Serial.write(','); Serial.println(y1, 3);

    // 5) ==== 20ms 동안 u 수신 폴링 (한 번만 온다고 가정) ====
    bool gotU = false;
    double u_tmp;
    while ((int32_t)(micros() - deadline) < 0) {
      if (!gotU && tryReadU(u_tmp)) {
        u_rx = u_tmp;
        gotU = true; // 첫 u만 사용
      }
      delayMicroseconds(150); // 과도한 바쁜-대기 방지
    }

    // 6) ==== 데드라인 도달: u 적용 ====
    double u_to_apply = gotU ? u_rx : u_last;
    if (!gotU) u_miss_count++;

    // 지연 시간 측정 (apply 시각)
    uint32_t apply_time = micros();
    uint32_t delay_us   = (uint32_t)((int32_t)(apply_time - t_calc));

    // 모터 구동
    double u_cmd = -u_to_apply; // 네거티브 피드백
    int pwmValue = constrain((int)fabs(u_cmd), 0, 255);
    bool forward = (u_cmd > 0.0);
    moveMotor(pwmValue, forward);

    u_last = u_to_apply;

    // 7) ==== 타이밍 통계 기록 ====
    delay_us_hist[stat_idx] = delay_us;
    uint32_t loop_us = have_prev_t ? (uint32_t)((int32_t)(t_calc - prev_t_calc)) : 0;
    loop_us_hist[stat_idx]  = loop_us;
    prev_t_calc = t_calc;
    have_prev_t = true;

    stat_idx++;
    if (stat_idx >= NSTAT) {
      // ★ 여기서만 요약을 한 번에 출력(쉼표 없음 → RPi는 무시)
      printTimingReportAndReset();
    }
  }
}
