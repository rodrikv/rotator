import requests
import fake_useragent

fug = fake_useragent.FakeUserAgent()

res = requests.get(
    "https://httpbin.org/ip",
    headers={"User-Agent": fug.random},
    proxies={"http": "http://127.0.0.1:8000", "https": "http://127.0.0.1:8000"},
    timeout=30,
)

print(res.text)
