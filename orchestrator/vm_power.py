import requests
import argparse
import json
import os
import sys
from dotenv import load_dotenv
import paramiko


VCENTER_URL = "https://vc.cs.illinois.edu/ui/mutation/applyOnMultiEntity"
REFERER_TEMPLATE = "https://vc.cs.illinois.edu/ui/app/vm;nav=h/{urn}/summary"

SEND_COMMAND_FLAG = False

# Load credentials from .env
load_dotenv()

HOST_BEGIN = os.getenv("SSH_HOST_BEGIN")
PORT = int(os.getenv("SSH_PORT", 22))
USERNAME = os.getenv("SSH_USER")
PASSWORD = os.getenv("SSH_PASS")

def format_hostname(vm_number: int) -> str:
    padded_num = f"{vm_number+1:02d}"
    return f"sp25-cs525-09{padded_num}.cs.illinois.edu"

def ssh_and_run(vm_number, cpu, mem):
    hostname = format_hostname(vm_number)
    print(f"Connecting to VM {vm_number} at {hostname}...")

    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(hostname, PORT, USERNAME, PASSWORD)

    shell = client.invoke_shell()
    commands = [
        "cd disaggregated_autoscale",
        f"GOTOOLCHAIN=auto go run server/server.go -cpu={cpu} -mem={mem}"
    ]

    for cmd in commands:
        print(f"Running on VM{vm_number}: {cmd}")
        shell.send(cmd + "\n")
        shell.recv(1024)

    # client.close()
    print(f"✅ Commands sent to VM{vm_number}")

def init_server(server_number):
    # send a command to that server to actially power on
    
    # get what the cpu and the mem of this vm is

    ssh_and_run(server_number, 20, 20)
    

def load_vm_config(path):
    with open(path, "r") as f:
        return json.load(f)

def load_cookies(path):
    with open(path, "r") as f:
        raw = json.load(f)
    return {cookie['name']: cookie['value'] for cookie in raw}

def build_payload(urn, state):
    global SEND_COMMAND_FLAG
    if state == "on":
        spec = "{\"powerState\":\"poweredOn\"}"
        SEND_COMMAND_FLAG = True
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

    # Base directory of the script
    script_dir = os.path.dirname(os.path.abspath(__file__))
    config_path = os.path.join(script_dir, args.config)
    cookies_path = os.path.join(script_dir, args.cookies)

    vm_config = load_vm_config(config_path)
    cookies = load_cookies(cookies_path)

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

    if SEND_COMMAND_FLAG:
        init_server(int(args.vm.split('-')[-1]))  # Converts e.g., vm-0912 to 912


if __name__ == "__main__":
    main()
