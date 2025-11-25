#!/usr/bin/env scriptling

print("Testing raw and triple-quoted strings...")

s1 = 'single quoted'
s2 = "double quoted"
print(s1, s2)

multi = '''line1
line2
line3'''
print('MULTI:', multi)

raw = r"a\b\c"
print('RAW:', raw)

# Regex style raw string
pattern = r'href=["\'](.*?)[\'"]'
print('PATTERN:', pattern)

print('Done')
