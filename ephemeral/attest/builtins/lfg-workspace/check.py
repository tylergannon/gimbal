from pathlib import Path
assert Path("greeting.txt").read_text() == "Gimble is ready.\n"
print("LFG_GREETING_OK")
