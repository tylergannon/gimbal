from pathlib import Path
assert Path("status.txt").read_text() == "ready\n"
print("GUIDED_LFG_OK")
