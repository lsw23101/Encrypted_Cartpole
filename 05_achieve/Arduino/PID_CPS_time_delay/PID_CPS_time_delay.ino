// ================== Arduino — RPi 통신 + 20ms 액츄에이터 지연 + 30ms 루프 ==================
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
const float ANGLE_OFFSET = 77.3 -1.73 + 34.0 -1.0; // 사용하신 오프셋

// ---------------- 타겟 ----------------
double targetAngle    = 0.0;
double targetPosition = 0.0;

// ---------------- 출력(y) / 입력(u) ----------------
double y[2] = {0.0, 0.0};  // y[0]=angleError, y[1]=positionError
double u_latest = 0.0;     // 가장 최근에 수신한 u
double u_applied = 0.0;    // 이번 스텝에 실제로 적용할 u

// ---------------- 타이밍 ----------------
unsigned long lastControlTime           = 0;
const unsigned long controlIntervalMs   = 30; // 전체 제어 주기 30ms
const unsigned long actuationDelayMs    = 20; // 액츄에이터 지연 20ms

bool isRunning = true;

// ---------------- 유틸 ----------------
inline float getCartDistanceM() {
  // 엔코더 255 CPR 가정
  return (encoderCount * 2.0f * PI) / 255.0f;
}

void moveMotor(int pwm, bool forward) {
  digitalWrite(MOTOR_DIR_PIN, forward ? HIGH : LOW);
  analogWrite(MOTOR_PWM_PIN, pwm);
}

void stopMotor() {
  analogWrite(MOTOR_PWM_PIN, 0);
}

// ---------------- 엔코더 ISR ----------------
void updateEncoder() {
  bool A = digitalRead(encoderPinA);
  bool B = digitalRead(encoderPinB);
  if (A == B) encoderCount++;
  else        encoderCount--;
}

// ---------------- 시리얼 수신(u) 파싱 ----------------
// 라즈베리에서 '\n'으로 끝나는 실수 문자열 전송 가정
bool tryReadU(double &u_out) {
  static char buf[16];
  static byte idx = 0;
  while (Serial.available() > 0) {
    char c = Serial.read();
    if (c == '\n' || c == '\r') {
      buf[idx] = '\0';
      idx = 0;
      u_out = atof(buf);
      return true;
    }
    if (idx < sizeof(buf) - 1) buf[idx++] = c;
  }
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

  Serial.println("Arduino ready. (30ms loop / 20ms actuation delay)");
  Serial.println("Commands: r(run/stop toggle), angle:<deg>, pos:<m>");
}

// ---------------- 메인 루프 ----------------
void loop() {
  // -------- 시리얼 명령 처리 --------
  // 숫자(u) 라인을 먹어버리지 않도록: '알파벳으로 시작하는 라인'만 명령으로 처리
  while (Serial.available() > 0) {
    int c = Serial.peek();
    if (c == 'r' || c == 'R') {              // 단일 토글
      Serial.read(); // 소비
      isRunning = !isRunning;
      if (!isRunning) {
        stopMotor();
        encoderCount = 0;
        u_applied = 0.0;
      }
      Serial.println(isRunning ? "System RUNNING." : "System STOPPED.");
      continue;
    } else if ((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')) {
      // 알파벳으로 시작 → 명령 라인 처리
      String line = Serial.readStringUntil('\n');
      line.trim();
      if (line.length() == 0) continue;

      line.toLowerCase();
      int colon = line.indexOf(':');
      if (colon > 0) {
        String key = line.substring(0, colon);
        String val = line.substring(colon + 1);
        double d = val.toFloat();

        if (key == "angle") {
          targetAngle = d;
          Serial.print("targetAngle set to "); Serial.println(targetAngle, 6);
        } else if (key == "pos" || key == "position") {
          targetPosition = d;
          Serial.print("targetPosition set to "); Serial.println(targetPosition, 6);
        }
      }
      // 알파벳이지만 콜론 없는 라인은 무시
    } else {
      // 숫자/부호/공백으로 시작 → 라즈베리의 u 일 가능성 큼 → 건드리지 않음
      break;
    }
  }

  // -------- 제어 루프(30ms) --------
  if (isRunning) {
    unsigned long now = millis();
    if (now - lastControlTime >= controlIntervalMs) {
      lastControlTime += controlIntervalMs;

      // 1) ADC -> 각도
      ADCvalue = analogRead(A1);
      currentAngle = (ADCvalue - ADCmin) * 360.0f / (ADCmax - ADCmin);
      currentAngle = constrain(currentAngle, 0.0f, 360.0f);

      // 오프셋 보정
      currentAngle += ANGLE_OFFSET;
      if (currentAngle < 0.0f)   currentAngle += 360.0f;
      if (currentAngle >= 360.0f) currentAngle -= 360.0f;

      // -180 ~ +180 변환
      float relativeAngle = currentAngle;
      if (relativeAngle > 180.0f) relativeAngle -= 360.0f;

      // 2) 오차 계산
      double angleError    = (double)targetAngle    - (double)relativeAngle;
      double positionError = (double)targetPosition - (double)getCartDistanceM();

      y[0] = angleError;
      y[1] = positionError;

      // 3) y 송신: "y0,y1\n"
      Serial.print(y[0], 6);
      Serial.print(",");
      Serial.println(y[1], 6);

      // 4) 액츄에이터 지연 20ms 동안 u 수신 폴링 (최신값으로 갱신)
      unsigned long t_start = millis();
      double u_tmp;
      while (millis() - t_start < actuationDelayMs) {
        if (tryReadU(u_tmp)) {
          u_latest = u_tmp; // 최신 u 갱신
        }
        delayMicroseconds(300);
      }

      // 5) 적용할 u 결정 및 모터 구동(네거티브 피드백)
      u_applied = u_latest;
      double u_cmd = -u_applied; // 네거티브 피드백
      int pwmValue = constrain((int)fabs(u_cmd), 0, 255);
      bool forward = (u_cmd > 0.0);
      moveMotor(pwmValue, forward);
    }
  }
}