import time

print("Starting training run...")
for epoch in range(1, 6):
    time.sleep(0.5)
    loss = 1.0 / epoch
    print(f"  epoch {epoch}/5  loss={loss:.4f}")
print("Done.")
