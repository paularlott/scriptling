import requests
import re
import html

url = 'https://getknot.dev/changelog'
html_text = requests.get(url).text

# Debug: Check if we got content
print(f"Fetched {len(html_text)} characters")
print("First 500 chars:", html_text[:500])
print()

# Split by h2 tags first, then process each section
sections = re.split(r'<h2[^>]*>', html_text, 0, re.I)

print(f"Found {len(sections)} sections")
print()

for section in sections[1:]:  # Skip first empty section before first h2
    # Extract title (everything before </h2>)
    title_match = re.search(r'^(.*?)</h2>', section, re.S | re.I)
    if not title_match:
        continue

    title = title_match.group(1)
    clean_title = html.unescape(re.sub(r'<[^>]+>', '', title))
    print('##', clean_title.strip())

    # Find all list items in this section
    items = re.findall(r'<li[^>]*>(.*?)</li>', section, re.S | re.I)
    for item in items:
        clean_item = html.unescape(re.sub(r'<[^>]+>', '', item))
        print('-', clean_item.strip())

    if items:
        print()
assert True