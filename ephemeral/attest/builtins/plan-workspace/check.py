from slug import slug

cases = {
    "": "", " ": "", "Hello": "hello", " Hello World ": "hello-world",
    "a\tb\nc": "a-b-c", "A   B": "a-b", "Keep! Punctuation?": "keep!-punctuation?",
    "a--b": "a--b", "123": "123", "ÉCOLE": "école", "\t\n": "", "x - y": "x---y",
}
for source, expected in cases.items():
    assert slug(source) == expected, (source, slug(source), expected)
print("SPRINT_SLUG_OK: 12 cases")
