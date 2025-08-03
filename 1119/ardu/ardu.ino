// ================== Arduino — RPi 통신 (30ms 루프 / 20ms 액츄에이터 지연) ==================
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
const float ANGLE_OFFSET = 180 -72.2; // 사용 중인 오프셋

// ---------------- 타겟 ----------------
double targetAngle    = 0.0; // 필요시 상위 시스템에서 추후 확장 가능 (현재는 0 기준)
double targetPosition = 0.0;

// ---------------- 출력(y) / 입력(u) ----------------
double y0_angleError = 0.0;      // y[0]
double y1_posError   = 0.0;      // y[1]
double u_applied     = 0.0;      // 마지막으로 적용된 u (타임아웃 시 유지)

// ---------------- 타이밍 ----------------
const unsigned long controlIntervalMs = 30; // 전체 제어 주기
const unsigned long actuationDelayMs  = 20; // 루프 시작→구동까지 고정 지연
unsigned long t_next = 0;                   // 다음 루프 기준시각(드리프트 제거)

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

// ---------------- ADC 평균 필터 함수 ----------------
// sampleCount가 클수록 잡음은 줄고 시간은 늘어납니다(루프 예산 고려).
float readFilteredADC(int pin, int sampleCount = 100) {
  long total = 0;
  for (int i = 0; i < sampleCount; i++) {
    total += analogRead(pin);
    delayMicroseconds(5);
  }
  return total / (float)sampleCount;
}

// ---------------- u 수신 (기간 내 1회만) ----------------
// '\n' 또는 '\r'로 끝나는 실수 문자열 1개를 기간 내에 받으면 true
bool readUSingleWithin(unsigned long windowMs, double &u_out) {
  static char buf[24];
  static byte idx = 0;

  unsigned long deadline = millis() + windowMs;
  while ((long)(deadline - millis()) > 0) {
    while (Serial.available() > 0) {
      char c = Serial.read();
      if (c == '\n' || c == '\r') {
        if (idx > 0) {
          buf[idx] = '\0';
          idx = 0;
          u_out = atof(buf);
          return true;  // 이번 루프에선 단 1회만 수신
        }
        idx = 0; // 빈 줄이면 초기화 후 계속
      } else {
        if (idx < sizeof(buf) - 1) buf[idx++] = c;
      }
    }
    delayMicroseconds(200);
  }
  idx = 0; // 타임아웃 시 버퍼 정리
  return false;
}

// ---------------- 설정 ----------------
void setup() {
  Serial.begin(115200);

  pinMode(MOTOR_DIR_PIN, OUTPUT);
  pinMode(MOTOR_PWM_PIN, OUTPUT);

  pinMode(encoderPinA, INPUT_PULLUP);
  pinMode(encoderPinB, INPUT_PULLUP);
  attachInterrupt(digitalPinToInterrupt(encoderPinA), updateEncoder, CHANGE);

  stopMotor();
  t_next = millis(); // 현재 시각부터 스케줄 시작

  // 필요시 초기 상태 알림 한 줄 (디버깅용)
  Serial.println("READY");
}

// ---------------- 메인 루프 ----------------
void loop() {
  // 스케줄된 30ms 타이밍에 맞춰 실행 (드리프트 제거)
  if ((long)(millis() - t_next) >= 0) {
    unsigned long t_start = t_next;
    t_next += controlIntervalMs;

    // ---------------- 센싱/전처리 (LPF 적용) ----------------
    // 1) ADC → 각도 (평균 필터 사용)
    ADCvalue = readFilteredADC(A1, 100); // 시간 예산에 맞춰 sampleCount 조절 가능
    currentAngle = (ADCvalue - ADCmin) * 360.0f / (ADCmax - ADCmin);
    currentAngle = constrain(currentAngle, 0.0f, 360.0f);

    // 2) 오프셋 보정
    currentAngle += ANGLE_OFFSET;
    if (currentAngle < 0.0f)    currentAngle += 360.0f;
    if (currentAngle >= 360.0f) currentAngle -= 360.0f;

    // 3) -180 ~ +180 변환
    float relativeAngle = currentAngle;
    if (relativeAngle > 180.0f) relativeAngle -= 360.0f;

    // 4) 각도 오차 (목표는 0)
    targetAngle = 0.0;
    double angleError = (double)targetAngle - (double)relativeAngle;

    // 5) 위치 오차
    double cartPosition  = (double)getCartDistanceM();
    double positionError = (double)targetPosition - cartPosition;

    // ---------------- y 송신 ----------------
    y0_angleError = angleError;
    y1_posError   = positionError;

    // 3) y 송신: "y0,y1\n" (소수 6자리)
    Serial.print(y0_angleError, 6);
    Serial.print(",");
    Serial.println(y1_posError, 6);

    // ---------------- actuationDelay 창 내 u 수신 ----------------
    // 루프 시작 시각 기준 정확히 20ms 지연 내에서 u 1회만 수신 시도
    double u_recv = u_applied;           // 기본값: 직전 u 유지
    bool got = readUSingleWithin(actuationDelayMs, u_recv);
    if (got) u_applied = u_recv;         // 수신되면 갱신

    // (정밀 지연) 루프 시작→20ms 도달 보장
    while ((long)(millis() - t_start) < (long)actuationDelayMs) {
      // 남은 수 μs 동안 바쁜대기 (짧은 sleep로 CPU 점유율 최소화)
      delayMicroseconds(50);
    }

    // ---------------- 모터 구동 ----------------
    // 네거티브 피드백
    double u_cmd = -u_applied;
    
    // 각도 오차 한계 보호: |y0| > 40° 이면 구동 중지
    if (fabs(y0_angleError) > 40.0) {
      u_cmd = 0.0;
    }

    int pwmValue = constrain((int)fabs(u_cmd), 0, 255);
    bool forward = (u_cmd > 0.0);
    moveMotor(pwmValue, forward);

    // ---------------- 30ms 정합 ----------------
    while ((long)(millis() - t_start) < (long)controlIntervalMs) {
      delayMicroseconds(100);
    }
  }
}
