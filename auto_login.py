# auto_login.py
from playwright.sync_api import sync_playwright
import json
import os

VCENTER_URL = "https://vc.cs.illinois.edu/ui"

USERNAME = os.getenv("UIUC_NETID")
PASSWORD = os.getenv("UIUC_PASSWORD")

def save_cookies(cookies, path="cookies.json"):
    with open(path, "w") as f:
        json.dump(cookies, f)

def login_and_save_cookies():
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        context = browser.new_context()
        page = context.new_page()

        print("🔐 Navigating to vCenter...")
        page.goto(VCENTER_URL, wait_until="networkidle")

        # Wait for the username field to appear
        page.wait_for_selector('#username', timeout=20000)

        # Fill in credentials
        page.fill('#username', USERNAME)
        page.fill('#password', PASSWORD)

        # Enable and click the login button via JS
        page.evaluate("document.getElementById('submit').disabled = false")
        page.click('#submit')

        # Wait until we're redirected to /ui
        page.wait_for_url("**/ui/**", timeout=30000)



        # Save cookies
        cookies = context.cookies()
        save_cookies(cookies)
        print("✅ cookies.json generated!")

        browser.close()

if __name__ == "__main__":
    if not USERNAME or not PASSWORD:
        print("❌ Please set UIUC_NETID and UIUC_PASSWORD as environment variables")
    else:
        login_and_save_cookies()
