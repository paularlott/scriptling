import re

# Test re.finditer returns list of Match objects
matches = re.finditer(r'\d+', 'abc123def456')
len(matches) == 2
matches[0].group(0) == '123'
matches[1].group(0) == '456'

# Test with compiled regex
pattern = re.compile(r'\w+')
matches = pattern.finditer('hello world test')
len(matches) == 3
matches[0].group(0) == 'hello'
matches[1].group(0) == 'world'
matches[2].group(0) == 'test'