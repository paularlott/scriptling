import requests, re
url='https://scriptling.dev'
resp=requests.get(url)
html=resp.text
pattern=re.compile(r'<a[^>]*href=["\']([^"\']+)["\'][^>]*>(.*?)</a>', re.S)
links=pattern.findall(html)
unique={}
for href,text in links:
    text=re.sub(r'<.*?>','',text).strip()
    if href not in unique:
        unique[href]=text
print(len(unique))
for href,text in unique.items():
    print(href,'|',text)
assert True