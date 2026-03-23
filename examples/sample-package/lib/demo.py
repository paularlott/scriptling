import strutils
import numutils

def run():
    print("=== sample package demo ===")
    print(strutils.slugify("Hello, World!"))
    print(strutils.truncate("A rather long piece of text that needs trimming", 20))
    print(numutils.clamp(150, 0, 100))
    print(numutils.lerp(0, 100, 0.25))
