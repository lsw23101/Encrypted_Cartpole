// ================== Arduino — RPi 통신(115200) + 20ms 지연 + 30ms 루프 + 프레임ID/수신분류 ==================
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
double angErr = 0.0;   // 각도 오차
double posErr = 0.0;   // 위치 오차
double u_rx   = 0.0;   // 이번 주기에 받은 u
double u_last = 0.0;   // 직전 주기 u (ZOH)

// ---------------- 타이밍 ----------------
unsigned long lastControlTime         = 0;
const unsigned long controlIntervalMs = 30;     // 주기 30ms
const unsigned long actuationDelayUs  = 20000;  // 지연 20ms

bool isRunning = true;

// ---------------- 프레임/진단 ----------------
uint32_t frameId = 0;               // y에 붙는 프레임 번호(증가)
uint32_t recv_dup_same_fid = 0;     // 같은 fid로 u가 2번 이상 옴
uint32_t recv_late_old_fid = 0;     // 옛날(fid < current) u 도착
uint32_t recv_future_fid   = 0;     // 미래(fid > current) u 도착(이상)
uint32_t miss_no_u_in_win  = 0;     // 20ms 창에서 u를 못 받은 횟수

// ---------------- 유틸 ----------------
inline float getCartDistanceM() {
  // 엔코더 255 CPR 가정
  return (encoderCount * 2.0f * PI) / 255.0f;
}

inline void moveMotor(int pwm, bool forward) {
  digitalWrite(MOTOR_DIR_PIN, forward ? HIGH : LOW);
  analogWrite(MOTOR_PWM_PIN, pwm);
}

inline void stopMotor() { analogWrite(MOTOR_PWM_PIN, 0); }

// ---------------- 엔코더 ISR ----------------
void updateEncoder() {
  bool A = digitalRead(encoderPinA);
  bool B = digitalRead(encoderPinB);
  if (A == B) encoderCount++; else encoderCount--;
}

// ---------------- 시리얼 수신(u) 파싱: "fid,u" 또는 단일 "u" 허용 ----------------
bool tryReadU(uint32_t &fid_out, double &u_out) {
  static char buf[40];
  static uint8_t idx = 0;

  while (Serial.available() > 0) {
    char c = Serial.read();
    if (c == '\r') continue;
    if (c == '\n') {
      buf[idx] = '\0'; idx = 0;
      if (buf[0] == '\0') continue;

      // 콤마가 있으면 "fid,u" 시도
      char *comma = strchr(buf, ',');
      if (comma) {
        *comma = '\0';
        char *s0 = buf;
        char *s1 = comma + 1;
        unsigned long fid = strtoul(s0, nullptr, 10);
        double uval = atof(s1);
        fid_out = (uint32_t)fid;
        u_out   = uval;
        return true;
      } else {
        // 구버전 호환: "u"만 온 경우 → 현재 프레임으로 간주(권장 X)
        fid_out = frameId;
        u_out   = atof(buf);
        return true;
      }
    }
    if (idx < sizeof(buf) - 1) buf[idx++] = c;
  }
  return false;
}

// ---------------- 설정 ----------------
void setup() {
  Serial.begin(115200);  // 고정
  pinMode(MOTOR_DIR_PIN, OUTPUT);
  pinMode(MOTOR_PWM_PIN, OUTPUT);
  pinMode(encoderPinA, INPUT_PULLUP);
  pinMode(encoderPinB, INPUT_PULLUP);
  attachInterrupt(digitalPinToInterrupt(encoderPinA), updateEncoder, CHANGE);

  // 쉼표 없는 안내 (라즈베리 파서 혼선 방지)
  Serial.println("Arduino ready @115200 30ms loop 20ms delay with fid");
}

// ---------------- 메인 루프 ----------------
void loop() {
  if (!isRunning) return;

  unsigned long now = millis();
  if (now - lastControlTime < controlIntervalMs) return;
  lastControlTime += controlIntervalMs;

  // 1) 각도 계산
  ADCvalue = analogRead(A1);
  currentAngle = (ADCvalue - ADCmin) * 360.0f / (ADCmax - ADCmin);
  currentAngle = constrain(currentAngle, 0.0f, 360.0f);
  currentAngle += ANGLE_OFFSET;
  if (currentAngle < 0.0f)    currentAngle += 360.0f;
  if (currentAngle >= 360.0f) currentAngle -= 360.0f;

  float relativeAngle = currentAngle;
  if (relativeAngle > 180.0f) relativeAngle -= 360.0f;

  // 2) 오차
  angErr = (double)targetAngle    - (double)relativeAngle;
  posErr = (double)targetPosition - (double)getCartDistanceM();

  // 3) 타임스탬프/데드라인
  uint32_t t_calc   = micros();
  uint32_t deadline = t_calc + (uint32_t)actuationDelayUs;

  // 4) y 전송: "fid,angErr,posErr"
  Serial.print(frameId); Serial.write(',');
  Serial.print(angErr, 3); Serial.write(',');
  Serial.println(posErr, 3);

  // 5) 20ms 창: u 수신(첫 수신만 채택), 분류 카운트
  bool gotU = false;
  double u_tmp = 0.0;
  uint32_t u_fid = 0;

  while ((int32_t)(micros() - deadline) < 0) {
    uint32_t f_in; double u_in;
    if (tryReadU(f_in, u_in)) {
      if (f_in == frameId) {
        if (!gotU) { gotU = true; u_tmp = u_in; u_fid = f_in; }
        else       { recv_dup_same_fid++; /* 중복 도착 */ }
      } else if (f_in < frameId) {
        recv_late_old_fid++;  // 늦게 도착한 이전 프레임의 u
      } else {
        recv_future_fid++;    // 미래 프레임의 u (이상)
      }
    }
    delayMicroseconds(150);
  }

  // 6) 적용
  double u_to_apply = gotU ? u_tmp : u_last;
  if (!gotU) miss_no_u_in_win++;

  double u_cmd = -u_to_apply; // 네거티브 피드백
  int pwmValue = constrain((int)fabs(u_cmd), 0, 255);
  bool forward = (u_cmd > 0.0);
  moveMotor(pwmValue, forward);
  u_last = u_to_apply;

  // 7) 다음 프레임
  frameId++;
}
