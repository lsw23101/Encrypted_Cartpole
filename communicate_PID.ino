const int MOTOR1_IN1_PIN = 12;
const int MOTOR1_IN2_PIN = 13;
const int MOTOR1_PWM_PIN = 3;

const byte ENCODER_A_PIN = 2;
const byte ENCODER_B_PIN = 7;

volatile long encoderCount = 0;

float ADCvalue = 0;
float currentAngle = 0;
const float ADCmin = 104.0;
const float ADCmax = 919.0;

const float PULSES_PER_REV = 2000.0;
const float GEAR_RATIO = 1.0;
const float WHEEL_RADIUS_CM = 7.0;
const float CM_PER_COUNT = (2.0 * 3.14159 * WHEEL_RADIUS_CM) / (PULSES_PER_REV * GEAR_RATIO);

unsigned long lastCommandTime = 0;
const unsigned long timeoutDuration = 200;

void setup() {
  Serial.begin(115200);

  pinMode(MOTOR1_IN1_PIN, OUTPUT);
  pinMode(MOTOR1_IN2_PIN, OUTPUT);
  pinMode(MOTOR1_PWM_PIN, OUTPUT);

  pinMode(ENCODER_A_PIN, INPUT_PULLUP);
  pinMode(ENCODER_B_PIN, INPUT_PULLUP);
  attachInterrupt(digitalPinToInterrupt(ENCODER_A_PIN), updateEncoder, CHANGE);
}

void loop() {
  ADCvalue = analogRead(A0);
  currentAngle = (ADCvalue - ADCmin) * 360.0 / (ADCmax - ADCmin);
  currentAngle = constrain(currentAngle, 0.0, 360.0);

  float relativeAngle = currentAngle;
  if (relativeAngle > 180.0) relativeAngle -= 360.0;

  float distance = encoderCount * CM_PER_COUNT;

  Serial.print(relativeAngle, 2);
  Serial.print(",");
  Serial.println(distance, 2);

  static String command = "";
  bool commandReceived = false;

  while (Serial.available()) {
    char c = Serial.read();
    if (c == '\n') {
      if (command == "reset") {
        encoderCount = 0;
      } else {
        int sepIndex = command.indexOf(',');
        if (sepIndex != -1) {
          int pwm = command.substring(0, sepIndex).toInt();
          int direction = command.substring(sepIndex + 1).toInt();
          moveMotor(pwm, direction == 1);
          lastCommandTime = millis();
          commandReceived = true;
        }
      }
      command = "";
    } else {
      command += c;
    }
  }

  if (!commandReceived && millis() - lastCommandTime > timeoutDuration) {
    stopMotor();
  }

  delay(50);
}

void updateEncoder() {
  int stateA = digitalRead(ENCODER_A_PIN);
  int stateB = digitalRead(ENCODER_B_PIN);
  if (stateA == stateB) encoderCount++;
  else encoderCount--;
}

void moveMotor(int pwm, bool direction) {
  pwm = constrain(pwm, 0, 255);
  analogWrite(MOTOR1_PWM_PIN, pwm);
  digitalWrite(MOTOR1_IN1_PIN, direction ? HIGH : LOW);
  digitalWrite(MOTOR1_IN2_PIN, direction ? LOW : HIGH);
}

void stopMotor() {
  analogWrite(MOTOR1_PWM_PIN, 0);
  digitalWrite(MOTOR1_IN1_PIN, LOW);
  digitalWrite(MOTOR1_IN2_PIN, LOW);
}
