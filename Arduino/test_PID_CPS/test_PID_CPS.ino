// === 아두이노 코드 (r로 시작/정지 토글) ===
// 모터 제어 핀
const int MOTOR_DIR_PIN = 12;
const int MOTOR_PWM_PIN = 11;

// 엔코더 핀
const int encoderPinA = 2;
const int encoderPinB = 7;
volatile long encoderCount = 0;

// ADC 센서
float ADCvalue = 0;
float currentAngle = 0;
const float ADCmin = 104.0;
const float ADCmax = 919.0;
const float ANGLE_OFFSET = 77.3 - 1.73;

// 타겟값
double targetAngle = 0.0;
double targetPosition = 0.0;

// 출력 벡터 y
double y[2] = {0.0, 0.0};

// 제어 입력 u (라즈베리에서 받음)
double u = 0.0;

unsigned long lastControlTime = 0;
const unsigned long controlInterval = 50; // 50ms
bool isRunning = false; // 전원 켜면 기본은 '대기'

// ---------- 유틸 ----------
void moveMotor(int pwm, bool forward) {
  digitalWrite(MOTOR_DIR_PIN, forward ? HIGH : LOW);
  analogWrite(MOTOR_PWM_PIN, pwm);
}

inline void stopEverything() {
  u = 0.0;
  y[0] = 0.0; y[1] = 0.0;
  moveMotor(0, false);
}

bool looksNumeric(const String& s) {
  // 공백/부호/소수점/지수표기(e/E)와 숫자만 허용
  for (unsigned int i = 0; i < s.length(); ++i) {
    char c = s[i];
    if (!((c >= '0' && c <= '9') || c == '+' || c == '-' || c == '.' || c == 'e' || c == 'E' || c == ' ' || c == '\t')) {
      return false;
    }
  }
  // 최소 한 자리 숫자는 포함되어야 유효로 간주
  for (unsigned int i = 0; i < s.length(); ++i) {
    if (s[i] >= '0' && s[i] <= '9') return true;
  }
  return false;
}

void handleSerial() {
  while (Serial.available() > 0) {
    String data = Serial.readStringUntil('\n');
    data.trim();
    if (data.length() == 0) return;

    // === r: 시작/정지 토글 ===
    if (data.equalsIgnoreCase("r")) {
      isRunning = !isRunning;
      stopEverything();        // 토글 즉시 u,y=0, 모터 OFF
      return;                  // 여분 문자가 있더라도 이번 루프는 여기서 정리
    }

    // === 숫자인 경우에만 제어입력으로 채택 (RUNNING일 때만) ===
    if (isRunning && looksNumeric(data)) {
      u = data.toDouble();
    }
    // 그 외 텍스트는 무시
  }
}

float getCartDistanceM() {
  return (encoderCount * 2 * PI) / 255.0; // 엔코더 255 CPR 기준
}

void updateEncoder() {
  bool A = digitalRead(encoderPinA);
  bool B = digitalRead(encoderPinB);
  if (A == B) encoderCount++;
  else encoderCount--;
}

// ---------- 표준 스케치 ----------
void setup() {
  Serial.begin(115200);

  pinMode(MOTOR_DIR_PIN, OUTPUT);
  pinMode(MOTOR_PWM_PIN, OUTPUT);

  pinMode(encoderPinA, INPUT_PULLUP);
  pinMode(encoderPinB, INPUT_PULLUP);
  attachInterrupt(digitalPinToInterrupt(encoderPinA), updateEncoder, CHANGE);

  stopEverything(); // 안전 정지로 시작
  // Serial.println("Arduino ready."); // 라즈베리 파서 간섭 방지 위해 비활성 권장
}

void loop() {
  // 1) 먼저 시리얼 처리 (r 토글/제어입력 수신)
  handleSerial();

  // 2) 50ms 주기 제어/송신
  if (millis() - lastControlTime >= controlInterval) {
    lastControlTime += controlInterval;

    if (isRunning) {
      // --- 센서 → 각도 ---
      ADCvalue = analogRead(A0);
      currentAngle = (ADCvalue - ADCmin) * 360.0 / (ADCmax - ADCmin);
      currentAngle = constrain(currentAngle, 0.0, 360.0);
      currentAngle += ANGLE_OFFSET;
      if (currentAngle < 0) currentAngle += 360.0;
      if (currentAngle >= 360.0) currentAngle -= 360.0;

      // -180 ~ 180
      float relativeAngle = currentAngle;
      if (relativeAngle > 180.0) relativeAngle -= 360.0;

      // --- 오차 계산 ---
      double angleError = targetAngle - relativeAngle;
      double positionError = targetPosition - getCartDistanceM();
      y[0] = angleError;
      y[1] = positionError;

      // --- 모터 구동 ---
      int pwmValue = constrain((int)abs(u), 0, 255);
      moveMotor(pwmValue, (-u > 0)); // negative feedback
    } else {
      // 정지 상태: y를 0,0 유지 & 모터 OFF
      stopEverything();
    }

    // 3) y 전송 (정지 상태에서도 0,0 지속 송신)
    Serial.print(y[0], 6);
    Serial.print(",");
    Serial.println(y[1], 6);

    // (u 에코 출력은 파서 간섭 방지 위해 하지 않음)
  }
}
