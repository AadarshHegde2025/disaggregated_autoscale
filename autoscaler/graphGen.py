import pandas as pd
import matplotlib.pyplot as plt

df = pd.read_csv('evaluation_test.csv')
plt.figure(figsize=(12, 6))
plt.plot(df["Value"], label="Brute Force", marker='o', alpha=0.7)
plt.plot(df["Greedy"], label="Greedy", marker='s', alpha=0.7)
plt.plot(df["Genetic"], label="Genetic", marker='^', alpha=0.7)

plt.title("Algorithm Comparison")
plt.xlabel("Instance")
plt.ylabel("Metric")
plt.legend()
plt.grid(True)
plt.tight_layout()
plt.savefig("algorithm_comparison.png")

df = pd.read_csv('iteration_test.csv')

plt.figure(figsize=(12, 6))
plt.plot(df["Value"], label="Brute Force", marker='o', alpha=0.7)
plt.plot(df["Greedy"], label="Greedy", marker='s', alpha=0.7)
plt.plot(df["Genetic"], label="Genetic", marker='^', alpha=0.7)

plt.title("Optimizer Type vs Iterations")
plt.xlabel("Instance")
plt.ylabel("Metric")
plt.legend()
plt.grid(True)
plt.tight_layout()
plt.savefig("algorithm_comparison.png")
