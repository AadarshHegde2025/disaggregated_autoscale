import requests
import argparse
import json
import os
import sys

VCENTER_URL = "https://vc.cs.illinois.edu/ui/mutation/applyOnMultiEntity"
REFERER_TEMPLATE = "https://vc.cs.illinois.edu/ui/app/vm;nav=h/{urn}/summary"

def load_vm_config(path):
    with open(path, "r") as f:
        return json.load(f)

def load_cookies(path):
    with open(path, "r") as f:
        raw = json.load(f)
    return {cookie['name']: cookie['value'] for cookie in raw}

def build_payload(urn, state):
    if state == "on":
        spec = "{\"powerState\":\"poweredOn\"}"
    elif state == "off":
        spec = "{\"powerOpType\":\"soft\",\"powerState\":\"poweredOff\"}"
    else:
        raise ValueError("Invalid state. Use 'on' or 'off'.")
    return {
        "objectIds": [urn],
        "propertyObjectType": "com.vmware.vsphere.client.vm.powerops.VmPowerStateSpec",
        "propertySpec": spec
    }

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--vm", required=True, help="VM ID (key from config file, e.g. vm-0912)")
    parser.add_argument("--state", required=True, choices=["on", "off"], help="Power state to set")
    parser.add_argument("--config", default="vcsphere_vm_nums.json", help="Path to VM ID config file")
    parser.add_argument("--cookies", default="cookies.json", help="Path to cookies file")
    args = parser.parse_args()

    vm_config = load_vm_config(args.config)
    cookies = load_cookies(args.cookies)

    if args.vm not in vm_config:
        print(f"❌ VM '{args.vm}' not found in {args.config}")
        sys.exit(1)

    urn = vm_config[args.vm]
    headers = {
        "Content-Type": "application/json;charset=utf-8",
        "X-VSPHERE-UI-XSRF-TOKEN": cookies.get("VSPHERE-UI-XSRF-TOKEN", ""),
        "Origin": "https://vc.cs.illinois.edu",
        "Referer": REFERER_TEMPLATE.format(urn=urn),
        "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:136.0) Gecko/20100101 Firefox/136.0"
    }

    payload = build_payload(urn, args.state)
    response = requests.post(VCENTER_URL, headers=headers, cookies=cookies, json=payload)

    print("✅ Status:", response.status_code)
    print("🔁 Response:", response.text)

if __name__ == "__main__":
    main()
