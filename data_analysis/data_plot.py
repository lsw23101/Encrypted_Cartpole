import pandas as pd
import matplotlib.pyplot as plt

# ===== 1. CSV 파일 로드 =====
csv_path = "data.csv"  # 실제 파일명으로 변경
df = pd.read_csv(csv_path)

# ===== 2. 기본 정보 출력 =====
print(f"Loaded {len(df)} samples from {csv_path}")
print(df.head())

# ===== 3. uDiff 최대값 계산 =====
u_diff_max = df["uDiff"].abs().max()
print(f"\n✅ Max |uDiff| = {u_diff_max:.6f}")

# ===== 4. X축: iteration =====
x = df["iter"]

# ===== 5. Plot 1: 출력값 (Angle & Position) =====
plt.figure(figsize=(8, 6))
plt.plot(x, df["y0_angle"], label="Angle (deg)")
plt.plot(x, df["y1_position"], label="Position (m)")
plt.ylabel("Output")
plt.title("Plant Outputs (Angle & Position)")
plt.legend()
plt.grid(True)
plt.tight_layout()
plt.savefig("plot_outputs.svg", format="svg")
print("💾 Saved: plot_outputs.svg")

# ===== 6. Plot 2: 제어입력 비교 (uLocal vs uRemote) =====
plt.figure(figsize=(8, 6))
plt.plot(x, df["uLocal"], label="u_Original",
         color="tab:blue", linewidth=2.5)
plt.plot(x, df["uRemote"], label="u_Encrypted",
         color="tab:orange", linewidth=1.5,
         linestyle=(0, (8, 6)))  # 느슨한 점선
plt.ylabel("Control Input")
plt.title("Control Input Comparison")
plt.legend()
plt.grid(True)
plt.tight_layout()
plt.savefig("plot_control.svg", format="svg")
print("💾 Saved: plot_control.svg")

# ===== 7. Plot 3: uDiff =====
plt.figure(figsize=(8, 6))
plt.plot(x, df["uDiff"], color="tab:green", label="uDiff = uLocal - uRemote")
plt.ylabel("uDiff")
plt.xlabel("Iteration")
plt.title("Control Difference")
plt.legend()
plt.grid(True)
plt.tight_layout()
plt.savefig("plot_udiff.svg", format="svg")
print("💾 Saved: plot_udiff.svg")

# ===== 8. 모든 창 표시 =====
plt.show()
