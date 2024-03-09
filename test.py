import requests
import fake_useragent
import os

fug = fake_useragent.FakeUserAgent()

os.environ['HTTP_PROXY'] = 'http://127.0.0.1:9000'
os.environ['HTTPS_PROXY'] = 'http://127.0.0.1:9000'
res = requests.get(
    "https://ipinfo.io/json",
    headers={"User-Agent": fug.random},
    # proxies={"http": "http://127.0.0.1:9000", "https": "http://127.0.0.1:9000"},
    timeout=20,
)

print(res.text)