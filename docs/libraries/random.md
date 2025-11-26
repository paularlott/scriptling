# Random Library

Random number generation functions.

## Functions

### random.randint(min, max)

Returns a random integer between min and max (inclusive).

**Parameters:**
- `min`: Minimum value (integer)
- `max`: Maximum value (integer)

**Returns:** Integer

**Example:**
```python
import random

num = random.randint(1, 100)
print(num)  # Random number between 1 and 100
```

### random.random()

Returns a random float between 0.0 and 1.0.

**Returns:** Float

**Example:**
```python
import random

num = random.random()
print(num)  # Random float like 0.123456
```

### random.choice(list)

Returns a random element from a list.

**Parameters:**
- `list`: List to choose from

**Returns:** Element from the list

**Example:**
```python
import random

fruits = ["apple", "banana", "cherry", "date"]
fruit = random.choice(fruits)
print(fruit)  # Random fruit from the list
```

### random.shuffle(list)

Shuffles a list in place.

**Parameters:**
- `list`: List to shuffle (modified in place)

**Returns:** None

**Example:**
```python
import random

cards = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
random.shuffle(cards)
print(cards)  # [3, 7, 1, 9, 2, 5, 8, 4, 6, 10] (random order)
```

## Usage Example

```python
import random

# Random integer
dice_roll = random.randint(1, 6)
print("Dice roll:", dice_roll)

# Random float
probability = random.random()
print("Probability:", probability)

# Random choice
colors = ["red", "green", "blue", "yellow", "purple"]
color = random.choice(colors)
print("Random color:", color)

# Shuffle a deck
deck = list(range(1, 53))  # Cards 1-52
random.shuffle(deck)
print("Shuffled deck:", deck[:5], "...")  # First 5 cards

# Generate random data
data = []
for i in range(10):
    data.append(random.randint(0, 100))

print("Random data:", data)
```