# sample package

A collection of string and numeric utility modules for Scriptling.

## Installation

```bash
scriptling --package sample.zip script.py
```

## Modules

- `strutils` — string manipulation utilities
- `numutils` — numeric helpers

## Quick Start

```python
import strutils
import numutils

print(strutils.slugify("Hello, World!"))   # hello-world
print(numutils.clamp(150, 0, 100))          # 100
```

## Running the Demo

```bash
scriptling --package sample.zip
```
