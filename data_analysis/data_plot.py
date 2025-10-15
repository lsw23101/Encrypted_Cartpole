import pandas as pd
import matplotlib.pyplot as plt

# ===== 1. CSV 파일 로드 =====
# 파일명은 실제 CSV 이름으로 변경하세요.
csv_path = "enc_plant_log_20251015_213455.csv"  # 예시 파일명
df = pd.read_csv(csv_path)

# ===== 2. 기본 정보 출력 =====
print(f"Loaded {len(df)} samples from {csv_path}")
print(df.head())

# ===== 3. uDiff 최대값 계산 =====
u_diff_max = df["uDiff"].abs().max()
print(f"\n✅ Max |uDiff| = {u_diff_max:.6f}")

# ===== 4. 시각화 =====
fig, axes = plt.subplots(3, 1, figsize=(10, 8), sharex=True)

x = df["iter"]  # iteration 번호 기준

# --- Plot 1: 출력값 (angle, position)
axes[0].plot(x, df["y0_angle"], label="Angle (deg)")
axes[0].plot(x, df["y1_position"], label="Position (m)")
axes[0].set_ylabel("Output")
axes[0].legend()
axes[0].set_title("Plant Outputs (Angle & Position)")

# --- Plot 2: 제어입력 비교 (uLocal vs uRemote)
axes[1].plot(x, df["uLocal"], label="uLocal (Plain)")
axes[1].plot(x, df["uRemote"], label="uRemote (Decrypted)", linestyle="--")
axes[1].set_ylabel("Control Input")
axes[1].legend()
axes[1].set_title("Control Input Comparison")

# --- Plot 3: uDiff
axes[2].plot(x, df["uDiff"], color="tab:red", label="uDiff = uLocal - uRemote")
axes[2].set_ylabel("uDiff")
axes[2].set_xlabel("Iteration")
axes[2].legend()
axes[2].set_title("Control Difference")

# ===== 5. 공통 설정 =====
for ax in axes:
    ax.grid(True)

plt.tight_layout()
plt.show()
