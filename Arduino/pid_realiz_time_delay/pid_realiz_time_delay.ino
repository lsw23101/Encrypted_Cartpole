// ================== 아두이노 — 상태공간+PID 튜너블 버전 ==================

// 모터 제어 핀 (MD10C 방식)
const int MOTOR_DIR_PIN = 12;
const int MOTOR_PWM_PIN = 11;

// 인터럽트 핀 (A상, B상 엔코더용)
const int encoderPinA = 2;
const int encoderPinB = 7;

// 엔코더 관련 변수
volatile long encoderCount = 0;

// 아날로그 각도 센서 변수
float ADCvalue = 0;
float currentAngle = 0;
const float ADCmin = 104.0;
const float ADCmax = 919.0;
const float ANGLE_OFFSET = 77.3 -1.73 + 34.0;

// 목표값 및 오차
double targetAngle = 0.0;
double targetPosition = 0.0;
double angleError = 0.0;
double positionError = 0.0;

// ================== (수정) 튜닝 가능한 PID/LPD 게인들 ==================
double Kp = 34, Ki = 2, Kd = 40;
double Lp = 40, Li = 0, Ld = 3;

// double Kp = 26, Ki = 1.4, Kd = 36;
// double Lp = 30, Li = 0, Ld = 5;

// (참고) 상태공간 A, B는 고정 — 현재 사용 X(문헌형식 유지)
const double A[4][4] = {
  {1, 0, 0, 0},
  {0, 0, 0, 0},
  {0, 0, 1, 0},
  {0, 0, 0, 0}
};

const double B[4][2] = {
  {1, 0},
  {1, 0},
  {0, 1},
  {0, 1}
};

// ================== (수정) C, D는 게인에 따라 동적 갱신 ==================
double C[1][4] = { {0, 0, 0, 0} }; // {Ki, -Kd, Li, -Ld}
double D[1][2] = { {0, 0} };       // {Kp+Ki+Kd, Lp+Li+Ld}

void updateGainMatrices() {
  C[0][0] = Ki;
  C[0][1] = -Kd;
  C[0][2] = Li;
  C[0][3] = -Ld;
  D[0][0] = Kp + Ki + Kd;
  D[0][1] = Lp + Li + Ld;
}

// 상태, 출력, 입력
double state[4][1] = {{0},{0},{0},{0}};
double y[2][1]     = {{0},{0}};
double u[1][1]     = {{0}};

// 타이머
unsigned long lastControlTime = 0;
// 제어 샘플링 타임 50ms
const unsigned long controlInterval = 30;

// (추가) 구동 지연 20ms: 제어 입력 인가 직전에 적용
const unsigned long actuationDelayMs = 20;

// 실행 상태
bool isRunning = true;

// 바퀴 정보 (사용 X)
const float wheelRadiusM = 0.04;
const float wheelCircumferenceM = 2 * PI * wheelRadiusM;
const float countsPerRevolution = 255.0;

// 거리 계산 함수(라디안 단위)
float getCartDistanceM() {
  return (encoderCount * 2 * PI) / countsPerRevolution;
}

float cartPosition = 0;

// ADC 평균 필터 함수
float readFilteredADC(int pin, int sampleCount = 100) {
  long total = 0;
  for (int i = 0; i < sampleCount; i++) {
    total += analogRead(pin);
    delayMicroseconds(5);
  }
  return total / (float)sampleCount;
}

// 시리얼 입력 처리용 버퍼
char serialBuffer[40];
byte bufferIndex = 0;

// --------- 유틸: 현재 게인 출력 ---------
void printGains() {
  Serial.print("Gains | ");
  Serial.print("Kp="); Serial.print(Kp, 6);
  Serial.print(", Ki="); Serial.print(Ki, 6);
  Serial.print(", Kd="); Serial.print(Kd, 6);
  Serial.print(" | Lp="); Serial.print(Lp, 6);
  Serial.print(", Li="); Serial.print(Li, 6);
  Serial.print(", Ld="); Serial.println(Ld, 6);
}

// --------- 명령 파서 ---------
void parseCommand(const char* cmdLine) {
  // 공백만 들어오면 무시
  if (!cmdLine || !cmdLine[0]) return;

  // 소문자 키만 비교하도록 간단 처리
  // (아두이노 String을 쓰지 않고 C 문자열로 처리)
  char buf[40];
  byte n = 0;
  for (; n < sizeof(buf)-1 && cmdLine[n]; n++) {
    char c = cmdLine[n];
    if (c >= 'A' && c <= 'Z') c = c - 'A' + 'a';
    buf[n] = c;
  }
  buf[n] = '\0';

  // run/stop/show 단일 명령
  if (!strcmp(buf, "r") || !strcmp(buf, "run")) {
    isRunning = !isRunning;
    if (!isRunning) {
      stopMotor();
      Serial.println("System STOPPED.");
      // 상태/신호 리셋
      state[0][0] = state[1][0] = state[2][0] = state[3][0] = 0;
      y[0][0] = y[1][0] = 0;
      u[0][0] = 0;
      encoderCount = 0;
      // 최종 게인 출력
      printGains();
    } else {
      Serial.println("System RUNNING.");
      printGains();
    }
    return;
  }
  if (!strcmp(buf, "stop")) {
    if (isRunning) {
      isRunning = false;
      stopMotor();
      Serial.println("System STOPPED.");
      state[0][0] = state[1][0] = state[2][0] = state[3][0] = 0;
      y[0][0] = y[1][0] = 0;
      u[0][0] = 0;
      encoderCount = 0;
    }
    printGains();
    return;
  }
  if (!strcmp(buf, "show")) {
    printGains();
    return;
  }

  // key:value 형태 파싱
  const char* colon = strchr(buf, ':');
  if (!colon) {
    Serial.print("Unknown command: ");
    Serial.println(buf);
    Serial.println("Use kp:, ki:, kd:, lp:, li:, ld:, show, run/stop or press 'r'");
    return;
  }

  // key, value 분리
  char key[8];
  double val = 0.0;
  {
    size_t keyLen = (size_t)(colon - buf);
    if (keyLen >= sizeof(key)) keyLen = sizeof(key) - 1;
    memcpy(key, buf, keyLen);
    key[keyLen] = '\0';
    val = atof(colon + 1);
  }

  bool updated = false;

  // 호환 키: p -> kp, pp -> lp
  if (!strcmp(key, "kp") || !strcmp(key, "p")) { Kp = val; updated = true; }
  else if (!strcmp(key, "ki")) { Ki = val; updated = true; }
  else if (!strcmp(key, "kd")) { Kd = val; updated = true; }
  else if (!strcmp(key, "lp") || !strcmp(key, "pp")) { Lp = val; updated = true; }
  else if (!strcmp(key, "li")) { Li = val; updated = true; }
  else if (!strcmp(key, "ld")) { Ld = val; updated = true; }
  else if (!strcmp(key, "angle")) { targetAngle = val; Serial.print("targetAngle set to "); Serial.println(targetAngle, 6); }
  else if (!strcmp(key, "pos") || !strcmp(key, "position")) { targetPosition = val; Serial.print("targetPosition set to "); Serial.println(targetPosition, 6); }
  else {
    Serial.print("Unknown key: ");
    Serial.println(key);
    Serial.println("Valid keys: kp, ki, kd, lp, li, ld, angle, pos");
  }

  if (updated) {
    updateGainMatrices();
    Serial.print("Updated ");
    Serial.print(key);
    Serial.print(" = ");
    Serial.println(val, 6);
    printGains();
  }
}

void setup() {
  Serial.begin(115200);

  pinMode(MOTOR_DIR_PIN, OUTPUT);
  pinMode(MOTOR_PWM_PIN, OUTPUT);

  pinMode(encoderPinA, INPUT_PULLUP);
  pinMode(encoderPinB, INPUT_PULLUP);
  attachInterrupt(digitalPinToInterrupt(encoderPinA), updateEncoder, CHANGE);

  updateGainMatrices();   // 초기 C,D 세팅

  Serial.println("Press 'r' to start/stop the control loop.");
  Serial.println("Tune gains e.g., 'kp:35', 'ki:4', 'kd:42', 'lp:40', 'li:0', 'ld:3'. Type 'show' to print gains.");
}

void loop() {
  // ------------------- 시리얼 입력 처리 -------------------
  while (Serial.available() > 0) {
    char c = Serial.read();

    // 줄바꿈으로 커맨드 확정
    if (c == '\n' || c == '\r') {
      if (bufferIndex > 0) {
        serialBuffer[bufferIndex] = '\0';
        parseCommand(serialBuffer);
        bufferIndex = 0;
      }
      continue;
    }

    // 단일 'r' 즉시 토글 (시리얼 모니터 라인엔딩 없음 모드 호환)
    if ((c == 'r' || c == 'R') && bufferIndex == 0) {
      parseCommand("r");
      continue;
    }

    // 버퍼 저장
    if (bufferIndex < sizeof(serialBuffer) - 1) {
      serialBuffer[bufferIndex++] = c;
    } else {
      // 버퍼 오버플로 막기
      serialBuffer[sizeof(serialBuffer) - 1] = '\0';
      parseCommand(serialBuffer);
      bufferIndex = 0;
    }
  }

  // ------------------- 제어 루프 -------------------
  if (isRunning && (millis() - lastControlTime >= controlInterval)) {
    lastControlTime += controlInterval;

    // 1) ADC → 각도
    ADCvalue = readFilteredADC(A1, 100);
    currentAngle = (ADCvalue - ADCmin) * 360.0 / (ADCmax - ADCmin);
    currentAngle = constrain(currentAngle, 0.0, 360.0);

    // 2) 오프셋 보정
    currentAngle += ANGLE_OFFSET;
    if (currentAngle < 0) currentAngle += 360.0;
    if (currentAngle >= 360.0) currentAngle -= 360.0;

    // 3) -180 ~ +180 변환
    float relativeAngle = currentAngle;
    if (relativeAngle > 180.0) relativeAngle -= 360.0;

    // 4) 각도 오차 (목표는 0)
    targetAngle = 0.0;
    angleError = targetAngle - relativeAngle;

    // 5) 위치 오차
    cartPosition = getCartDistanceM();
    positionError = targetPosition - cartPosition;

    // 출력 벡터 y = [angleError, positionError]^T
    y[0][0] = angleError;
    y[1][0] = positionError;

    // ---- 제어 입력 u = C*x + D*y ----
    u[0][0] =
        C[0][0] * state[0][0] + C[0][1] * state[1][0] +
        C[0][2] * state[2][0] + C[0][3] * state[3][0] +
        D[0][0] * y[0][0]     + D[0][1] * y[1][0];

    // ================== (추가) 인가 지연 20 ms ==================
    // 이전 스텝의 모터 출력이 이 지연 동안 유지되며,
    // 지연 후 새 u가 반영됩니다.
    delay(actuationDelayMs);

    // 모터 구동 (부호로 방향, 크기로 PWM)
    double uCmd = -u[0][0]; // 기존 코드와 동일한 방향성 유지
    int pwmValue = constrain((int)fabs(uCmd), 0, 255);
    moveMotor(pwmValue, (uCmd > 0));

    // 디버그 출력
    Serial.print("y[0]="); Serial.print(y[0][0], 6);
    Serial.print(", y[1]="); Serial.print(y[1][0], 6);
    Serial.print(" | u=");   Serial.println(uCmd, 6);

    // ---- 상태 업데이트 (적분/미분 상태) ----
    state[0][0] = state[0][0] + y[0][0];
    state[1][0] =               y[0][0];
    state[2][0] = state[2][0] + y[1][0];
    state[3][0] =               y[1][0];
  }
}

// ------------------- 인터럽트/모터 함수들 -------------------

// 엔코더 인터럽트
void updateEncoder() {
  bool A = digitalRead(encoderPinA);
  bool B = digitalRead(encoderPinB);
  if (A == B) {
    encoderCount++;
  } else {
    encoderCount--;
  }
}

// 모터 제어
void moveMotor(int pwm, bool forward) {
  digitalWrite(MOTOR_DIR_PIN, forward ? HIGH : LOW);
  analogWrite(MOTOR_PWM_PIN, pwm);
}

// 모터 정지
void stopMotor() {
  analogWrite(MOTOR_PWM_PIN, 0);
}
