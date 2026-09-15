import re


def slug(text: str) -> str:
    stripped = text.strip()
    if not stripped:
        return ""
    return re.sub(r"\s+", "-", stripped.lower())
